package state

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils"
)

//go:generate mockgen -source state_delay_model.go -destination=state_delay_model_mock.go -package=state

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

// txConnectionKey uniquely identifies a directed connection between two
// addresses.
type txConnectionKey struct {
	from types.Address
	to   types.Address
}

// === FixedProcessingDelayModel ===

// FixedProcessingDelayModel is a fixed delay model with base and custom delays,
// supporting asymmetric per-connection transaction delays.
type FixedProcessingDelayModel struct {
	txDelay                *utils.DelayModel[txConnectionKey]
	blockFinalizationDelay *utils.DelayModel[uint32]
}

// NewFixedProcessingDelayModel creates a new fixed processing delay model with
// no initial delays.
func NewFixedProcessingDelayModel() *FixedProcessingDelayModel {
	return &FixedProcessingDelayModel{
		txDelay:                utils.NewDelayModel[txConnectionKey](),
		blockFinalizationDelay: utils.NewDelayModel[uint32](),
	}
}

// --- Transaction Delay ---

// SetBaseTransactionDelay sets a delay applied to all transactions.
func (m *FixedProcessingDelayModel) SetBaseTransactionDelay(
	delay time.Duration,
) {
	m.txDelay.ConfigureBase(utils.FixedDelay(delay))
}

// SetConnectionTransactionDelay sets a custom delay for transactions from one
// address to another, overriding the base transaction delay.
func (m *FixedProcessingDelayModel) SetConnectionTransactionDelay(
	from,
	to types.Address,
	delay time.Duration,
) {
	m.txDelay.ConfigureCustom(txConnectionKey{from: from, to: to}, utils.FixedDelay(delay))
}

// GetTransactionDelay returns the processing delay for a transaction.
func (m *FixedProcessingDelayModel) GetTransactionDelay(
	tx types.Transaction,
) time.Duration {
	return m.txDelay.GetDelay(txConnectionKey{from: tx.From, to: tx.To})
}

// --- Block Finalization Delay ---

// SetBaseBlockFinalizationDelay sets a delay applied to all block
// finalizations.
func (m *FixedProcessingDelayModel) SetBaseBlockFinalizationDelay(
	delay time.Duration,
) {
	m.blockFinalizationDelay.ConfigureBase(utils.FixedDelay(delay))
}

// SetCustomBlockFinalizationDelay sets a custom delay for finalizing a specific
// block number, overriding the base finalization delay.
func (m *FixedProcessingDelayModel) SetCustomBlockFinalizationDelay(
	blockNumber uint32,
	delay time.Duration,
) {
	m.blockFinalizationDelay.ConfigureCustom(blockNumber, utils.FixedDelay(delay))
}

// GetBlockFinalizationDelay returns the finalization delay for a block number.
func (m *FixedProcessingDelayModel) GetBlockFinalizationDelay(
	blockNumber uint32,
	txs []types.Transaction,
) time.Duration {
	return m.blockFinalizationDelay.GetDelay(blockNumber)
}

// === SampledProcessingDelayModel ===

// SampledProcessingDelayModel implements a processing delay model that samples
// delays from log-normal distributions, providing realistic processing delay
// simulation with natural variation and occasional long-tail delays. The base
// transaction distribution is used for all transactions that don't have custom
// distributions, while custom distributions can be set for specific
// connections asymmetrically. The same applies to block finalization delays.
type SampledProcessingDelayModel struct {
	txDistribution                *utils.DelayModel[txConnectionKey]
	blockFinalizationDistribution *utils.DelayModel[uint32]
}

// NewSampledProcessingDelayModel creates a new sampled processing delay model
// with default log-normal distributions for both transaction and block
// finalization delays. timeUnit specifies the unit for the sampled delays
// (e.g., time.Millisecond).
func NewSampledProcessingDelayModel() *SampledProcessingDelayModel {
	return &SampledProcessingDelayModel{
		txDistribution:                utils.NewDelayModel[txConnectionKey](),
		blockFinalizationDistribution: utils.NewDelayModel[uint32](),
	}
}

// SetBaseTransactionDistribution sets a log-normal distribution used for
// sampling transaction delays for all connections that don't have custom
// distributions.
func (m *SampledProcessingDelayModel) SetBaseTransactionDistribution(
	dist utils.Distribution,
) {
	m.txDistribution.ConfigureBase(dist)
}

// GetBaseTransactionDistribution returns the base transaction distribution, or
// nil if not configured.
func (m *SampledProcessingDelayModel) GetBaseTransactionDistribution() utils.Distribution {
	return m.txDistribution.GetBase()
}

// SetConnectionTransactionDistribution sets a custom log-normal distribution
// for sampling transaction delays from one address to another, overriding the
// base distribution.
func (m *SampledProcessingDelayModel) SetConnectionTransactionDistribution(
	from, to types.Address,
	dist utils.Distribution,
) {
	m.txDistribution.ConfigureCustom(txConnectionKey{from: from, to: to}, dist)
}

// GetTransactionDelay returns a sampled transaction delay for a transaction.
func (m *SampledProcessingDelayModel) GetTransactionDelay(
	tx types.Transaction,
) time.Duration {
	return m.txDistribution.GetDelay(txConnectionKey{from: tx.From, to: tx.To})
}

// SetBaseBlockFinalizationDistribution sets a log-normal distribution used for
// sampling block finalization delays for all blocks that don't have custom
// distributions.
func (m *SampledProcessingDelayModel) SetBaseBlockFinalizationDistribution(
	dist utils.Distribution,
) {
	m.blockFinalizationDistribution.ConfigureBase(dist)
}

// GetBaseBlockFinalizationDistribution returns the base block finalization
// distribution, or nil if not configured.
func (m *SampledProcessingDelayModel) GetBaseBlockFinalizationDistribution() utils.Distribution {
	return m.blockFinalizationDistribution.GetBase()
}

// SetCustomBlockFinalizationDistribution sets a custom log-normal distribution
// for sampling finalization delays for a specific block number, overriding the
// base distribution.
func (m *SampledProcessingDelayModel) SetCustomBlockFinalizationDistribution(
	blockNumber uint32,
	dist utils.Distribution,
) {
	m.blockFinalizationDistribution.ConfigureCustom(blockNumber, dist)
}

// GetBlockFinalizationDelay returns a sampled finalization delay for a block.
func (m *SampledProcessingDelayModel) GetBlockFinalizationDelay(
	blockNumber uint32,
	txs []types.Transaction,
) time.Duration {
	return m.blockFinalizationDistribution.GetDelay(blockNumber)
}
