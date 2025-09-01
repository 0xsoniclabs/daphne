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

// Factory defines the configuration for the central consensus algorithm
// instance.
type Factory struct {
	// EmitInterval is the interval at which the leader emits new bundles.
	EmitInterval time.Duration
}

// NewActive creates a new active central consensus instance.
// source is used to get candidate transactions for the next bundle.
func (f Factory) NewActive(server p2p.Server,
	source consensus.TransactionProvider) *Central {
	return NewActiveCentral(server, source, &f)
}

// NewPassive creates a new passive central consensus instance.
// This instance does not create/emit bundles but listens for them
// from the leader.
func (f Factory) NewPassive(server p2p.Server) *Central {
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
	processedBundles      map[uint32]struct{}
	processedBundlesMutex sync.Mutex

	nextBundleNumber uint32

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
		&emissionPayloadSourceAdapter{transactionSource: source, central: res},
		res.gossip,
		config.EmitInterval,
	)

	return res
}

// NewPassiveCentral creates a new passive central consensus instance.
// This instance does not emit bundles but listens for them from the leader.
func NewPassiveCentral(server p2p.Server, config *Factory) *Central {
	res := &Central{
		p2p:              server,
		config:           config,
		processedBundles: make(map[uint32]struct{}),
	}
	gossip := generic.NewGossip(
		server,
		func(message BundleMessage) uint32 {
			return message.Bundle.Number
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

// BundleMessage is the message type used for broadcasting new bundles.
// It contains the bundle number so that peers can track which bundles they have
// seen in addition to the bundle itself.
type BundleMessage struct {
	Bundle types.Bundle
}

// addBundle processes a bundle locally, broadcasts it to peers,
// and notifies listeners.
func (c *Central) addBundle(bundleMsg BundleMessage) {
	c.processedBundlesMutex.Lock()
	// Check if we've already processed this bundle locally - if so, ignore
	if _, alreadyProcessed :=
		c.processedBundles[bundleMsg.Bundle.Number]; alreadyProcessed {
		c.processedBundlesMutex.Unlock()
		return
	}

	// Mark bundle as processed locally
	c.processedBundles[bundleMsg.Bundle.Number] = struct{}{}
	c.processedBundlesMutex.Unlock()

	slog.Info("Processing bundle", "blockNumber", bundleMsg.Bundle.Number)

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
			Number:       c.nextBundleNumber,
			Transactions: transactions,
		},
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
