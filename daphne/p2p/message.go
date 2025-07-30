package p2p

//go:generate stringer -type=MessageCode -output message_string.go -trimprefix MessageCode_

// MessageCode is an enumeration type for identifying P2P messages.
type MessageCode int

const (
	// Message codes for a simple protocol used in unit tests.
	MessageCode_UnitTestProtocol_Ping MessageCode = 100
)

// Message is a message being forwarded between peers in the P2P network.
type Message struct {
	Code    MessageCode
	Payload any
}
