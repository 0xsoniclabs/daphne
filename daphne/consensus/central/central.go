package central

import (
	"log/slog"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

const (
	// DefaultEmitInterval is the default interval for emitting new bundles
	// if one is not specified in the configuration.
	DefaultEmitInterval = 500 * time.Millisecond
)

// Factory defines the configuration for the central consensus algorithm
// instance.
type Factory struct {
	// EmitInterval is the interval at which the leader emits new bundles.
	EmitInterval time.Duration
}

// NewActive creates a new active central consensus instance.
// source is used to get candidate transactions for the next bundle.
func (f Factory) NewActive(server p2p.Server,
	source consensus.TransactionProvider) consensus.Consensus {
	return NewActiveCentral(server, source, &f)
}

// NewPassive creates a new passive central consensus instance.
// This instance does not create/emit bundles but listens for them
// from the leader.
func (f Factory) NewPassive(server p2p.Server) consensus.Consensus {
	return NewPassiveCentral(server, &f)
}

// Central implements the central consensus algorithm.
// It is responsible for coordinating the consensus process, broadcasting new
// bundles, and handling incoming bundle messages from peers.
type Central struct {
	p2p            p2p.Server
	listenersMutex sync.Mutex
	listeners      []consensus.BundleListener
	config         *Factory

	// Track locally processed bundles to avoid duplicate processing.
	processedBundles map[bundleNumber]struct{}
	nextBundleNumber bundleNumber

	gossip  generic.Gossip[BundleMessage]
	emitter *generic.Emitter[BundleMessage]
}

// NewActiveCentral creates a new active central consensus instance.
// It creates and emits new bundles at a configured interval, using the
// provided source to get candidate transactions.
func NewActiveCentral(
	server p2p.Server,
	source consensus.TransactionProvider,
	config *Factory,
) *Central {
	res := NewPassiveCentral(server, config)
	res.emitter = generic.StartEmitter(
		config.EmitInterval,
		&emissionPayloadSourceAdapter{transactionSource: source, central: res},
		res.gossip,
	)

	return res
}

// NewPassiveCentral creates a new passive central consensus instance.
// This instance does not emit bundles but listens for them from the leader.
func NewPassiveCentral(server p2p.Server, config *Factory) *Central {
	res := &Central{
		p2p:              server,
		config:           config,
		processedBundles: make(map[bundleNumber]struct{}),
	}
	gossip := generic.NewGossip(
		server,
		func(message BundleMessage) bundleNumber {
			return message.Number
		},
		p2p.MessageCode_CentralConsensus_NewBundle,
	)
	gossip.RegisterReceiver(&onMessageAdapter{central: res})
	res.gossip = gossip
	return res
}

// RegisterListener registers a new bundle listener to receive notifications
// about new bundles emitted by the central consensus algorithm's leader.
func (c *Central) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		c.listenersMutex.Lock()
		defer c.listenersMutex.Unlock()
		c.listeners = append(c.listeners, listener)
	}
}

// Stop stops the active central consensus instance and its bundle emission.
// It blocks until the emission loop exits.
func (c *Central) Stop() {
	if c.emitter != nil {
		c.emitter.Stop()
		c.emitter = nil
	}
}

type bundleNumber uint32

// BundleMessage is the message type used for broadcasting new bundles.
// It contains the bundle number so that peers can track which bundles they have
// seen in addition to the bundle itself.
type BundleMessage struct {
	Number bundleNumber
	Bundle types.Bundle
}

// addBundle processes a bundle locally, broadcasts it to peers,
// and notifies listeners.
func (c *Central) addBundle(bundleMsg BundleMessage) {
	// Check if we've already processed this bundle locally - if so, ignore
	if _, alreadyProcessed :=
		c.processedBundles[bundleMsg.Number]; alreadyProcessed {
		return
	}

	// Mark bundle as processed locally
	c.processedBundles[bundleMsg.Number] = struct{}{}
	slog.Info("Processing bundle", "blockNumber", bundleMsg.Number)

	// Broadcast to peers
	c.gossip.Broadcast(bundleMsg)

	// Notify local listeners
	c.listenersMutex.Lock()
	defer c.listenersMutex.Unlock()
	for _, listener := range c.listeners {
		listener.OnNewBundle(bundleMsg.Bundle)
	}
}

func (c *Central) nextBundleMessage(transactions []types.Transaction) BundleMessage {
	bundleMessage := BundleMessage{
		Bundle: types.Bundle{
			Transactions: transactions,
		},
		Number: c.nextBundleNumber,
	}
	// Process the bundle locally before giving it to the Emitter.
	c.addBundle(bundleMessage)
	c.nextBundleNumber++
	return bundleMessage
}

type emissionPayloadSourceAdapter struct {
	transactionSource consensus.TransactionProvider
	central           *Central
}

func (e *emissionPayloadSourceAdapter) GetEmissionPayload() BundleMessage {
	return e.central.nextBundleMessage(e.transactionSource.GetCandidateTransactions())
}

type onMessageAdapter struct {
	central *Central
}

func (m *onMessageAdapter) OnMessage(msg BundleMessage) {
	m.central.addBundle(msg)
}
