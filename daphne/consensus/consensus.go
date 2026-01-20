package consensus

//go:generate mockgen -source consensus.go -destination=consensus_mock.go -package=consensus

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
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
	// Stop stops all active components of the consensus instance and blocks
	// until all goroutines owned by the instance exit. If no active
	// components are running, the method returns immediately.
	Stop()
}

// Factory defines the methods required to instantiate a consensus protocol
// implementation.
type Factory interface {
	// Every consensus instance, whether active or passive, emits bundles.
	// NewActive creates a new active consensus instance. Only active consensus
	// instances contribute to the decision-making, or consensus, to reach a
	// quorum to both create and emit new bundles.
	NewActive(p2p.Server, Committee, ValidatorId, TransactionProvider) Consensus
	// NewPassive creates a new passive consensus instance. A passive consensus
	// instance is a listener without decision-making power in consensus,
	// that is, it only observes, and potentially broadcasts, messages from the
	// rest of the network to reach the same conclusions/recalculate which
	// bundles to emit that active members would propose/create.
	NewPassive(p2p.Server, Committee) Consensus
	// Stringer is required to make factories usable in logging and reporting.
	fmt.Stringer
}

// TransactionProvider is a component returning information about the current
// state of the chain and candidate transactions for upcoming blocks.
// TODO: consider rename to XXXProvider?
// TODO: move to payload package
type TransactionProvider interface {
	GetCurrentBlockNumber() uint32
	GetCandidateLineup() txpool.Lineup
}

// BundleListener is an interface that defines the common behaviour expected
// from listeners such as block processors or other components.
type BundleListener interface {
	OnNewBundle(bundle types.Bundle)
}

func WrapBundleListener(f func(bundle types.Bundle)) bundleListener {
	return bundleListener(f)
}

type bundleListener func(types.Bundle)

func (b bundleListener) OnNewBundle(bundle types.Bundle) {
	b(bundle)
}
