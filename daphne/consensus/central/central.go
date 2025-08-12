package central

import (
	"log/slog"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
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
	p2p       p2p.Server
	listeners []consensus.BundleListener
	config    *Factory

	// Track seen bundles by peer ID and bundle number.
	seenBundles map[p2p.PeerId]map[bundleNumber]struct{}

	// quit and done channels are used to stop the active central consensus
	// instance. quit is used to signal the emitter ticker goroutine to stop,
	// and done is closed when the main goroutine finishes.
	quit chan<- struct{}
	done <-chan struct{}
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

	quit := make(chan struct{})
	done := make(chan struct{})
	nextBlock := bundleNumber(0)

	// Use either configured emit interval or default
	emitInterval := config.EmitInterval
	if emitInterval == 0 {
		emitInterval = DefaultEmitInterval
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(emitInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				transactions := source.GetCandidateTransactions()
				slog.Info("Emitting new bundle", "blockNumber", nextBlock,
					"transactions", len(transactions))

				// Pack a new bundle with the candidate transactions.
				bundle := types.Bundle{
					Transactions: transactions,
				}

				// Notify all local registered listeners about the new bundle.
				res.broadcast(BundleMessage{
					Number: nextBlock,
					Bundle: bundle,
				})
				nextBlock++

			// Keep emitting until we are signaled to stop.
			case <-quit:
				return
			}
		}
	}()
	res.quit = quit
	res.done = done
	return res
}

// NewPassiveCentral creates a new passive central consensus instance.
// This instance does not emit bundles but listens for them from the leader.
func NewPassiveCentral(server p2p.Server, config *Factory) *Central {
	c := &Central{
		p2p:         server,
		config:      config,
		seenBundles: make(map[p2p.PeerId]map[bundleNumber]struct{}),
	}

	// Register message handlers.
	server.RegisterMessageHandler(&messageHandlerAdapter{central: c})
	return c
}

// RegisterListener registers a new bundle listener to receive notifications
// about new bundles emitted by the central consensus algorithm's leader.
func (c *Central) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		c.listeners = append(c.listeners, listener)
	}
}

// Stop stops the active central consensus instance.
// It closes the quit channel to signal the emitter goroutine to stop,
// and waits for the done channel to be closed before returning.
func (c *Central) Stop() {
	if c.quit != nil {
		close(c.quit)
		c.quit = nil
		<-c.done
		c.done = nil
	}
}

// handleMessage processes incoming messages from peers.
// It checks if the message is a new bundle and broadcasts it to all peers.
// It also notifies local listeners about the new bundle.
func (c *Central) handleMessage(sender p2p.PeerId, msg p2p.Message) {
	// Only handle messages that are of type NewBundle.
	if msg.Code != p2p.MessageCode_CentralConsensus_NewBundle {
		return
	}

	incoming, ok := msg.Payload.(BundleMessage)
	// Validate the incoming message.
	// If the payload is not of type bundleMessage, log a warning and return.
	// This ensures that we only process valid bundle messages.
	if !ok {
		slog.Warn("Received invalid bundle message", "peerId", sender, "payload",
			msg.Payload)
		return
	}
	slog.Info("Received new bundle message", "at", c.p2p.GetLocalId(), "from",
		sender, "blockNumber", incoming.Number)

	// Track seen bundles by peer ID and bundle number to avoid rebroadcasting
	// bundles that have already been seen by peers.
	if seen := c.seenBundles[sender]; seen == nil {
		c.seenBundles[sender] = make(map[bundleNumber]struct{})
	}
	c.seenBundles[sender][incoming.Number] = struct{}{}

	c.broadcast(incoming)

	// Notify all local registered listeners about the new bundle.
	for _, listener := range c.listeners {
		listener.OnNewBundle(incoming.Bundle)
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

// broadcast sends the given bundle message to all peers that haven't seen it
// yet.
func (c *Central) broadcast(message BundleMessage) {
	msg := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: message,
	}

	// Broadcast the bundle to all peers that haven't seen it yet.
	for _, peer := range c.p2p.GetPeers() {
		if c.seenBundles[peer] == nil {
			c.seenBundles[peer] = make(map[bundleNumber]struct{})
		}
		if _, seen := c.seenBundles[peer][message.Number]; !seen {
			c.seenBundles[peer][message.Number] = struct{}{}
			err := c.p2p.SendMessage(peer, msg)
			if err != nil {
				slog.Error("Failed to send message", "peerId", peer, "error", err)
			}
		}
	}
}

// messageHandlerAdapter is an adapter that implements the p2p.MessageHandler
// interface for the Central consensus algorithm.
type messageHandlerAdapter struct {
	central *Central
}

// HandleMessage is called by the p2p server when a new message is received.
// It delegates the handling of the message to the central consensus instance.
func (m *messageHandlerAdapter) HandleMessage(peerId p2p.PeerId,
	msg p2p.Message) {
	m.central.handleMessage(peerId, msg)
}
