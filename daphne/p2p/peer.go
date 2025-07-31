package p2p

// PeerId is a unique identifier for a peer in the P2P network.
type PeerId string

// peer is the interface exposed by each peer to the network to manage the
// peers connections and to facilitate message passing.
type peer interface {
	connectTo(peerId PeerId)
	receiveMessage(from PeerId, msg Message)
}
