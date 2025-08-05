package consensus

//go:generate mockgen -source consensus.go -destination=consensus_mock.go -package=consensus

import (
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

type Algorithm interface {
	NewActive(p2p.Server, PayloadSource) Consensus
	NewPassive(p2p.Server) Consensus
}

type PayloadSource interface {
	GetCandidateTransactions() []types.Transaction
}

type Consensus interface {
	RegisterListener(listener BundleListener)
}

type BundleListener interface {
	OnNewBundle(bundle types.Bundle)
}
