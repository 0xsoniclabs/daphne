package state

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils"
)

// ProcessingDelayModel defines how delays are applied during state processing.
// GetTransactionDelay returns the execution delay for an individual
// transaction. GetBlockFinalizationDelay returns the delay for finalizing a
// block.
type ProcessingDelayModel interface {
	GetTransactionDelay(tx types.Transaction) time.Duration
	GetBlockFinalizationDelay(
		blockNumber uint32,
		txs []types.Transaction,
	) time.Duration
}

type txConnectionKey struct {
	from types.Address
	to   types.Address
}

// === FixedProcessingDelayModel ===

// FixedProcessingDelayModel is a fixed delay model with base and custom delays,
// supporting asymmetric per-connection transaction delays.
type FixedProcessingDelayModel struct {
	txDelay                utils.DelayModel[types.Address, time.Duration]
	blockFinalizationDelay utils.DelayModel[uint32, time.Duration]
}

// NewFixedProcessingDelayModel creates a new fixed processing delay model with
// no initial delays.
func NewFixedProcessingDelayModel() *FixedProcessingDelayModel {
	return &FixedProcessingDelayModel{
		txDelay:                utils.NewFixedDelayModel[types.Address, time.Duration](),
		blockFinalizationDelay: utils.NewFixedDelayModel[uint32, time.Duration](),
	}
}

// --- Transaction Delay ---

// SetBaseTransactionDelay sets a delay applied to all transactions.
func (m *FixedProcessingDelayModel) SetBaseTransactionDelay(
	delay time.Duration,
) {
	m.txDelay.SetBaseDelay(delay)
}

// SetConnectionTransactionDelay sets a custom delay for transactions from one
// address to another, overriding the base transaction delay.
func (m *FixedProcessingDelayModel) SetConnectionTransactionDelay(
	from,
	to types.Address,
	delay time.Duration,
) {
	m.txDelay.SetCustomDelay(from, to, delay)
}

// GetTransactionDelay returns the processing delay for a transaction.
func (m *FixedProcessingDelayModel) GetTransactionDelay(
	tx types.Transaction,
) time.Duration {
	return m.txDelay.GetDelay(tx.From, tx.To)
}

// --- Block Finalization Delay ---

// SetBaseBlockFinalizationDelay sets a delay applied to all block
// finalizations.
func (m *FixedProcessingDelayModel) SetBaseBlockFinalizationDelay(
	delay time.Duration,
) {
	m.blockFinalizationDelay.SetBaseDelay(delay)
}

// SetCustomBlockFinalizationDelay sets a custom delay for finalizing a specific
// block number, overriding the base finalization delay.
func (m *FixedProcessingDelayModel) SetCustomBlockFinalizationDelay(
	blockNumber uint32,
	delay time.Duration,
) {
	m.blockFinalizationDelay.SetCustomDelay(blockNumber, 0, delay)
}

// GetBlockFinalizationDelay returns the finalization delay for a block number.
func (m *FixedProcessingDelayModel) GetBlockFinalizationDelay(
	blockNumber uint32,
	txs []types.Transaction,
) time.Duration {
	return m.blockFinalizationDelay.GetDelay(blockNumber, 0)
}

// === SampledProcessingDelayModel ===

// SampledProcessingDelayModel implements a processing delay model that samples
// delays from log-normal distributions, providing realistic processing delay
// simulation with natural variation and occasional long-tail delays. The base
// transaction distribution is used for all transactions that don't have custom
// distributions, while custom distributions can be set for specific
// connections asymmetrically. The same applies to block finalization delays.
type SampledProcessingDelayModel struct {
	txDistribution                utils.DelayModel[types.Address, *utils.LogNormalDistribution]
	blockFinalizationDistribution utils.DelayModel[uint32, *utils.LogNormalDistribution]

	// timeUnit defines the unit for sampled delays (e.g., time.Millisecond)
	timeUnit time.Duration
}

// NewSampledProcessingDelayModel creates a new sampled processing delay model
// with default log-normal distributions for both transaction and block
// finalization delays. timeUnit specifies the unit for the sampled delays
// (e.g., time.Millisecond).
func NewSampledProcessingDelayModel(timeUnit time.Duration) *SampledProcessingDelayModel {
	return &SampledProcessingDelayModel{
		txDistribution:                utils.NewFixedDelayModel[types.Address, *utils.LogNormalDistribution](),
		blockFinalizationDistribution: utils.NewFixedDelayModel[uint32, *utils.LogNormalDistribution](),
		timeUnit:                      timeUnit,
	}
}

// SetBaseTransactionDistribution sets a log-normal distribution used for
// sampling transaction delays for all connections that don't have custom
// distributions.
func (m *SampledProcessingDelayModel) SetBaseTransactionDistribution(
	mu,
	sigma float64,
	seed *int64,
) {
	m.txDistribution.SetBaseDelay(utils.NewLogNormalDistribution(mu, sigma, seed))
}

// SetConnectionTransactionDistribution sets a custom log-normal distribution
// for sampling transaction delays from one address to another, overriding the
// base distribution.
func (m *SampledProcessingDelayModel) SetConnectionTransactionDistribution(
	from, to types.Address,
	mu,
	sigma float64,
	seed *int64,
) {
	m.txDistribution.SetCustomDelay(from, to, utils.NewLogNormalDistribution(mu, sigma, seed))
}

// GetTransactionDelay returns a sampled transaction delay for a transaction.
func (m *SampledProcessingDelayModel) GetTransactionDelay(
	tx types.Transaction,
) time.Duration {
	dist := m.txDistribution.GetDelay(tx.From, tx.To)
	if dist == nil {
		return 0
	}
	return dist.SampleDuration(m.timeUnit)
}

// SetBaseBlockFinalizationDistribution sets a log-normal distribution used for
// sampling block finalization delays for all blocks that don't have custom
// distributions.
func (m *SampledProcessingDelayModel) SetBaseBlockFinalizationDistribution(
	mu,
	sigma float64,
	seed *int64,
) {
	m.blockFinalizationDistribution.SetBaseDelay(utils.NewLogNormalDistribution(mu, sigma, seed))
}

// SetCustomBlockFinalizationDistribution sets a custom log-normal distribution
// for sampling finalization delays for a specific block number, overriding the
// base distribution.
func (m *SampledProcessingDelayModel) SetCustomBlockFinalizationDistribution(
	blockNumber uint32,
	mu,
	sigma float64,
	seed *int64,
) {
	m.blockFinalizationDistribution.SetCustomDelay(blockNumber, 0, utils.NewLogNormalDistribution(mu, sigma, seed))
}

// GetBlockFinalizationDelay returns a sampled finalization delay for a block.
func (m *SampledProcessingDelayModel) GetBlockFinalizationDelay(
	blockNumber uint32,
	txs []types.Transaction,
) time.Duration {
	dist := m.blockFinalizationDistribution.GetDelay(blockNumber, 0)
	if dist == nil {
		return 0
	}
	return dist.SampleDuration(m.timeUnit)
}
