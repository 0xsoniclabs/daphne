package txpool

import (
	"log/slog"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

type TxGossip struct {
	pool TxPool
	p2p  p2p.Server

	knownTransactions map[p2p.PeerId]map[types.Hash]struct{}
}

func NewTxGossip(pool TxPool, p2pServer p2p.Server) *TxGossip {
	res := &TxGossip{
		pool:              pool,
		p2p:               p2pServer,
		knownTransactions: make(map[p2p.PeerId]map[types.Hash]struct{}),
	}
	pool.RegisterListener(poolListenerAdapter{res})
	p2pServer.RegisterMessageHandler(poolMessageHandlerAdapter{res})
	return res
}

func (g *TxGossip) handleMessage(from p2p.PeerId, msg p2p.Message) {
	if msg.Code != p2p.MessageCode_TxGossip_NewTransaction {
		return
	}

	tx, ok := msg.Payload.(types.Transaction)
	if !ok {
		slog.Warn("Received invalid transaction payload", "payload", msg.Payload)
		return
	}

	if _, exists := g.knownTransactions[from]; !exists {
		g.knownTransactions[from] = make(map[types.Hash]struct{})
	}
	g.knownTransactions[from][tx.Hash()] = struct{}{}

	if err := g.pool.Add(tx); err != nil {
		return
	}

	g.broadcastTransaction(tx)
}

func (g *TxGossip) onNewTransaction(tx types.Transaction) {
	g.broadcastTransaction(tx)
}

func (g *TxGossip) broadcastTransaction(tx types.Transaction) {
	for _, peer := range g.p2p.GetPeers() {
		if _, exists := g.knownTransactions[peer]; !exists {
			g.knownTransactions[peer] = make(map[types.Hash]struct{})
		}

		if _, exists := g.knownTransactions[peer][tx.Hash()]; !exists {
			g.knownTransactions[peer][tx.Hash()] = struct{}{}
			err := g.p2p.SendMessage(peer, p2p.Message{
				Code:    p2p.MessageCode_TxGossip_NewTransaction,
				Payload: tx,
			})
			if err != nil {
				slog.Warn("Failed to send transaction gossip", "peer", peer, "hash", tx.Hash(), "error", err)
			}
		}
	}
}

type poolListenerAdapter struct {
	gossip *TxGossip
}

func (a poolListenerAdapter) OnNewTransaction(tx types.Transaction) {
	a.gossip.onNewTransaction(tx)
}

type poolMessageHandlerAdapter struct {
	gossip *TxGossip
}

func (a poolMessageHandlerAdapter) HandleMessage(from p2p.PeerId, msg p2p.Message) {
	a.gossip.handleMessage(from, msg)
}
