package txpool

import (
	"log/slog"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// InstallTxGossip installs a synchronization protocol automatically keeping the
// given pool in sync with other pools on the P2P network running the same protocol.
// The employed synchronization protocol is a best-effort-only protocol. It does not
// provide any guarantees on consistency or the timing of pool updates.
func InstallTxGossip(pool TxPool, p2pServer p2p.Server) {
	txGossip := &txGossip{
		pool:                     pool,
		p2p:                      p2pServer,
		transactionsKnownByPeers: make(map[p2p.PeerId]map[types.Hash]struct{}),
	}
	pool.RegisterListener(txGossip)
	p2pServer.RegisterMessageHandler(txGossip)
}

// txGossip is a protocol for gossiping transactions across remote Transaction pools in the network.
// It listens for new transactions in the local transaction pool and broadcasts them to connected peers.
// It also handles incoming transaction messages from remote peers, adding them to the local transaction pool.
type txGossip struct {
	pool TxPool
	p2p  p2p.Server

	// transactionsKnownByPeers keeps track of transactions known by each peer
	// It is updated when a specific transaction is received from or sent to a specific peer.
	transactionsKnownByPeers      map[p2p.PeerId]map[types.Hash]struct{} // peer -> transaction hash
	transactionsKnownByPeersMutex sync.Mutex
}

func (g *txGossip) HandleMessage(from p2p.PeerId, msg p2p.Message) {
	if msg.Code != p2p.MessageCode_TxGossip_NewTransaction {
		return
	}
	tx, ok := msg.Payload.(types.Transaction)
	if !ok {
		slog.Warn("Received invalid transaction payload", "payload", msg.Payload)
		return
	}
	g.markTransactionKnownByPeer(from, tx.Hash())
	if err := g.pool.Add(tx); err != nil {
		slog.Info("Received transaction not added to pool", "sender", from,
			"transaction hash", tx.Hash(), "reason", err)
	}
	g.broadcastTransaction(tx)
}

func (g *txGossip) OnNewTransaction(tx types.Transaction) {
	g.broadcastTransaction(tx)
}

func (g *txGossip) broadcastTransaction(tx types.Transaction) {
	for _, peer := range g.p2p.GetPeers() {
		if g.isTransactionKnownByPeer(peer, tx.Hash()) {
			continue
		}
		g.markTransactionKnownByPeer(peer, tx.Hash())
		err := g.p2p.SendMessage(peer, p2p.Message{
			Code:    p2p.MessageCode_TxGossip_NewTransaction,
			Payload: tx,
		})
		if err != nil {
			slog.Warn("Failed to send transaction gossip", "sender", peer,
				"transaction hash", tx.Hash(), "error", err)
		}
	}
}

// isTransactionKnownByPeer returns true if the transaction
// is new for the specified peer. It is thread-safe.
func (g *txGossip) isTransactionKnownByPeer(peer p2p.PeerId, txHash types.Hash) bool {
	g.transactionsKnownByPeersMutex.Lock()
	defer g.transactionsKnownByPeersMutex.Unlock()
	if _, exists := g.transactionsKnownByPeers[peer]; !exists {
		return false
	}
	if _, exists := g.transactionsKnownByPeers[peer][txHash]; !exists {
		return false
	}
	return true
}

// markTransactionKnownByPeer updates the known transactions for a
// specified peer. It is thread-safe.
func (g *txGossip) markTransactionKnownByPeer(peer p2p.PeerId, txHash types.Hash) {
	g.transactionsKnownByPeersMutex.Lock()
	defer g.transactionsKnownByPeersMutex.Unlock()
	if _, exists := g.transactionsKnownByPeers[peer]; !exists {
		g.transactionsKnownByPeers[peer] = make(map[types.Hash]struct{})
	}
	if _, exists := g.transactionsKnownByPeers[peer][txHash]; !exists {
		g.transactionsKnownByPeers[peer][txHash] = struct{}{}
	}
}
