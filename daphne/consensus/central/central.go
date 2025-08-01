package central

import (
	"log/slog"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

const (
	EmitInterval = 500 * time.Millisecond
)

type Algorithm struct{}

func (a Algorithm) NewActive(server p2p.Server, source consensus.PayloadSource) consensus.Consensus {
	return NewActiveCentral(server, source)
}

func (a Algorithm) NewPassive(server p2p.Server) consensus.Consensus {
	return NewPassiveCentral(server)
}

type Central struct {
	p2p       p2p.Server
	listeners []consensus.BundleListener

	seenBundles map[p2p.PeerId]map[uint32]struct{} // Track seen bundles by peer ID and bundle number

	quit chan<- struct{}
	done <-chan struct{}
}

func NewActiveCentral(
	server p2p.Server,
	source consensus.PayloadSource,
) *Central {
	res := NewPassiveCentral(server)
	quit := make(chan struct{})
	done := make(chan struct{})
	nextBlock := uint32(0)
	go func() {
		defer close(done)
		ticker := time.NewTicker(EmitInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				transactions := source.GetCandidateTransactions()
				slog.Info("Emitting new bundle", "blockNumber", nextBlock, "transactions", len(transactions))

				// Emit a new bundle with transactions from the pool.
				bundle := types.Bundle{
					Transactions: transactions,
				}

				res.broadcast(bundleMessage{
					Number: nextBlock,
					Bundle: bundle,
				})
				nextBlock++

			case <-quit:
				return
			}
		}
	}()
	res.quit = quit
	res.done = done
	return res
}

func NewPassiveCentral(server p2p.Server) *Central {
	c := &Central{
		p2p:         server,
		seenBundles: make(map[p2p.PeerId]map[uint32]struct{}),
	}

	// Register message handlers.
	server.RegisterMessageHandler(&messageHandlerAdapter{central: c})
	return c
}

func (c *Central) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		c.listeners = append(c.listeners, listener)
	}
}

func (c *Central) Stop() {
	if c.quit != nil {
		close(c.quit)
		c.quit = nil
		<-c.done
		c.done = nil
	}
}

func (c *Central) handleMessage(sender p2p.PeerId, msg p2p.Message) {
	if msg.Code != p2p.MessageCode_CentralConsensus_NewBundle {
		return
	}

	incoming, ok := msg.Payload.(bundleMessage)
	if !ok {
		slog.Warn("Received invalid bundle message", "peerId", sender, "payload", msg.Payload)
		return
	}
	slog.Info("Received new bundle message", "at", c.p2p.GetLocalId(), "from", sender, "blockNumber", incoming.Number)

	if seen := c.seenBundles[sender]; seen == nil {
		c.seenBundles[sender] = make(map[uint32]struct{})
	}
	c.seenBundles[sender][incoming.Number] = struct{}{}

	c.broadcast(incoming)

	// Notify all local registered listeners about the new bundle.
	for _, listener := range c.listeners {
		listener.OnNewBundle(incoming.Bundle)
	}
}

type bundleMessage struct {
	Number uint32
	Bundle types.Bundle
}

func (c *Central) broadcast(message bundleMessage) {
	msg := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: message,
	}

	// Broadcast the bundle to all peers that haven't seen it yet.
	for _, peer := range c.p2p.GetPeers() {
		if c.seenBundles[peer] == nil {
			c.seenBundles[peer] = make(map[uint32]struct{})
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

type messageHandlerAdapter struct {
	central *Central
}

func (m *messageHandlerAdapter) HandleMessage(peerId p2p.PeerId, msg p2p.Message) {
	m.central.handleMessage(peerId, msg)
}
