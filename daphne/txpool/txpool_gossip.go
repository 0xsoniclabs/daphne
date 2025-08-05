package txpool

import (
	"log/slog"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// TxGossip is a protocol for gossiping transactions across remote Transaction pools in the network.
// It listens for new transactions in the local transaction pool and broadcasts them to connected peers.
// It also handles incoming transaction messages from remote peers, adding them to the local transaction pool.
type TxGossip struct {
	pool TxPool
	p2p  p2p.Server

	// transactionsKnownByPeers keeps track of transactions known by each peer
	// It is updated when a specific transaction is received from or sent to a specific peer.
	transactionsKnownByPeers      map[p2p.PeerId]map[types.Hash]struct{} // peer -> transaction hash
	transactionsKnownByPeersMutex sync.Mutex
}

// InstallTxGossip initializes the transaction gossip protocol by registering
// listeners on the transaction pool and the P2P server.
func InstallTxGossip(pool TxPool, p2pServer p2p.Server) {
	txGossip := &TxGossip{
		pool:                     pool,
		p2p:                      p2pServer,
		transactionsKnownByPeers: make(map[p2p.PeerId]map[types.Hash]struct{}),
	}
	pool.RegisterListener(poolListenerAdapter{txGossip})
	p2pServer.RegisterMessageHandler(poolMessageHandlerAdapter{txGossip})
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
	g.updateTransactionsKnownByPeer(from, tx.Hash())
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
		if !g.updateTransactionsKnownByPeer(peer, tx.Hash()) {
			continue
		}
		err := g.p2p.SendMessage(peer, p2p.Message{
			Code:    p2p.MessageCode_TxGossip_NewTransaction,
			Payload: tx,
		})
		if err != nil {
			slog.Warn("Failed to send transaction gossip", "peer", peer,
				"hash", tx.Hash(), "error", err)
		}
	}
}

// updateTransactionsKnownByPeer attempts to update the known transactions for a
// specified peer and returns true if the transaction is new for that peer. Is thread-safe.
func (g *TxGossip) updateTransactionsKnownByPeer(peer p2p.PeerId, txHash types.Hash) bool {
	g.transactionsKnownByPeersMutex.Lock()
	defer g.transactionsKnownByPeersMutex.Unlock()
	if _, exists := g.transactionsKnownByPeers[peer]; !exists {
		g.transactionsKnownByPeers[peer] = make(map[types.Hash]struct{})
	}
	if _, exists := g.transactionsKnownByPeers[peer][txHash]; !exists {
		g.transactionsKnownByPeers[peer][txHash] = struct{}{}
		return true
	}
	return false
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
