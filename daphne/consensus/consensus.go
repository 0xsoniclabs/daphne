package consensus

//go:generate mockgen -source consensus.go -destination=consensus_mock.go -package=consensus

import (
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// Consensus defines the common functionality expected from all consensus
// protocol implementations.
type Consensus interface {
	// RegisterListener registers a listener which is notified on bundle creation.
	// Bundles are the common output of any consensus protocol. This is
	// implemented as a callback because each implementation may commit bundles
	// based on different event inputs and/or timing. The fact that consensus
	// will eventually be reached is the only common interface.
	RegisterListener(listener BundleListener)
}

// Factory defines the methods required to instantiate a consensus protocol
// implementation.
type Factory interface {
	// NewActive creates a new active consensus instance that emits bundles
	// using the provided source transaction provide to get candidate
	// transactions.
	NewActive(p2p.Server, TransactionProvider) Consensus
	// NewPassive creates a new passive consensus instance that does not emit
	// bundles. It can verify and linearize bundles but it does not contribute
	// to bundle creation.
	NewPassive(p2p.Server) Consensus
}

// TransactionProvider is a component that returns candidate transactions
// for linearization in the consensus protocol.
type TransactionProvider interface {
	GetCandidateTransactions() []types.Transaction
}

// BundleListener is an interface that defines the common behaviour expected
// from listeners such as block processors or other components.
type BundleListener interface {
	OnNewBundle(bundle types.Bundle)
}
