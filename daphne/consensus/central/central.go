package central

import (
	"log/slog"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

const (
	EmitInterval = 500 * time.Millisecond
)

type Central struct {
	p2p       p2p.Server
	listeners []BundleListener

	seenBundles map[p2p.PeerId]map[uint32]struct{} // Track seen bundles by peer ID and bundle number

	quit chan<- struct{}
	done <-chan struct{}
}

func NewActiveCentral(
	server p2p.Server,
	state state.State,
	pool txpool.TxPool,
) *Central {
	res := NewPassiveCentral(server)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(EmitInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				source := nonceSourceAdapter{state: state}
				lastBlock := state.GetCurrentBlockNumber()
				transactions := pool.GetExecutableTransactions(source)
				slog.Info("Emitting new bundle", "blockNumber", lastBlock+1, "transactions", len(transactions))

				// Emit a new bundle with transactions from the pool.
				bundle := types.Bundle{
					Number:       lastBlock + 1,
					Transactions: transactions,
				}

				res.broadcast(bundle)
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

func (c *Central) RegisterListener(listener BundleListener) {
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

type BundleListener interface {
	OnNewBundle(bundle types.Bundle)
}

func (c *Central) handleMessage(sender p2p.PeerId, msg p2p.Message) {
	if msg.Code != p2p.MessageCode_CentralConsensus_NewBundle {
		return
	}

	bundle, ok := msg.Payload.(types.Bundle)
	if !ok {
		slog.Warn("Received invalid bundle message", "peerId", sender, "payload", msg.Payload)
		return
	}
	slog.Info("Received new bundle message", "at", c.p2p.GetLocalId(), "from", sender, "blockNumber", bundle.Number)

	if seen := c.seenBundles[sender]; seen == nil {
		c.seenBundles[sender] = make(map[uint32]struct{})
	}
	c.seenBundles[sender][bundle.Number] = struct{}{}

	c.broadcast(bundle)

	// Notify all local registered listeners about the new bundle.
	for _, listener := range c.listeners {
		listener.OnNewBundle(bundle)
	}
}

func (c *Central) broadcast(bundle types.Bundle) {
	msg := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: bundle,
	}

	// Broadcast the bundle to all peers that haven't seen it yet.
	for _, peer := range c.p2p.GetPeers() {
		if c.seenBundles[peer] == nil {
			c.seenBundles[peer] = make(map[uint32]struct{})
		}
		if _, seen := c.seenBundles[peer][bundle.Number]; !seen {
			c.seenBundles[peer][bundle.Number] = struct{}{}
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

type nonceSourceAdapter struct {
	state state.State
}

func (n nonceSourceAdapter) GetNonce(address types.Address) types.Nonce {
	account := n.state.GetAccount(address)
	return account.Nonce
}
