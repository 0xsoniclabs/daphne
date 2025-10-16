package p2p

import (
	"reflect"
)

// Message represents a generic P2P message that can be sent between peers in
// the network. Right now, no requirements are imposed on messages, but in
// the future, we may want to require that messages implement certain methods
// for serialization, validation, etc.
type Message any

// MessageType is a string identifier for the type of message.
type MessageType string

// GetMessageType returns the type identifier for a given message. If the
// message implements the TypedMessage interface, its MessageType method is
// used. Otherwise, the name of the message's Go type is returned.
func GetMessageType(msg Message) MessageType {
	if typedMsg, ok := msg.(TypedMessage); ok {
		return typedMsg.MessageType()
	}
	return MessageType(reflect.TypeOf(msg).Name())
}

// TypedMessage is an interface for messages that can provide their own type
// identifier. If implemented, this identifier is used by GetMessageType instead
// of the default name.
type TypedMessage interface {
	Message
	MessageType() MessageType
}
