// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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

// GetMessageSize returns the size of a given message. If the message implements
// the SizedMessage interface, its MessageSize method is used. Otherwise, it
// returns 0 to indicate that the size is unknown.
func GetMessageSize(msg Message) uint32 {
	if sizedMsg, ok := msg.(SizedMessage); ok {
		return sizedMsg.MessageSize()
	}
	// This message does not tell us its size.
	return 0
}

// SizedMessage is an interface for messages that can provide their own size
// in bytes. If implemented, this size is used by GetMessageSize instead of the
// default of 0.
type SizedMessage interface {
	Message
	MessageSize() uint32
}
