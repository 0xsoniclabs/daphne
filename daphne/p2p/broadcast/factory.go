package broadcast

import (
	"github.com/0xsoniclabs/daphne/daphne/p2p"
)

// Factory is a factory function type for creating new Channel instances.
// It is used to allow dependency injection of different Channel
// implementations in components like the TxPool or Consensus modules.
//
// A call to the factory function should return a new instance of a Channel
// using the provided p2p.Server and may use the provided extractKeyFromMessage
// to determine unique keys for messages of type M.
type Factory[K comparable, M any] func(
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
) Channel[M]
