package p2p

//go:generate stringer -type=MessageCode -output message_string.go -trimprefix MessageCode_

// MessageCode is an enumeration type for identifying P2P messages.
type MessageCode int

const (
	// Message codes for a simple protocol used in unit tests.
	MessageCode_UnitTestProtocol_Ping MessageCode = iota

	// --- TxGossip messages ---

	// MessageCode_TxGossip_NewTransaction announces a new transaction to peers.
	MessageCode_TxGossip_NewTransaction

	// --- Central Consensus messages ---
	MessageCode_CentralConsensus_NewBundle
)

// Message is a message being forwarded between peers in the P2P network.
type Message struct {
	Code    MessageCode
	Payload any
}
