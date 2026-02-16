// Copyright 2026 Sonic Labs
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

// PeerId is a unique identifier for a peer in the P2P network.
type PeerId string

// peer is the interface exposed by each peer to the network to manage the
// peers connections and to facilitate message passing.
type peer interface {
	connectTo(peerId PeerId)
	clearConnections()
	receiveMessage(from PeerId, msg Message)
}
