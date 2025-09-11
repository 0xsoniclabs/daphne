package state

import (
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

// ProcessingDelayModel defines how delays are applied during state processing.
// GetTransactionDelay returns the execution delay for an individual transaction.
// GetBlockFinalizationDelay returns the delay for finalizing a block.
type ProcessingDelayModel interface {
	GetTransactionDelay(tx types.Transaction) time.Duration
	GetBlockFinalizationDelay(blockNumber uint32) time.Duration
}

// FixedProcessingDelayModel is a fixed delay model with base and custom delays,
// supporting asymmetric per-connection transaction delays.
type FixedProcessingDelayModel struct {
	baseTxDelay             time.Duration
	customTxDelays          map[txConnectionKey]time.Duration
	baseFinalizationDelay   time.Duration
	customFinalizationDelay map[uint32]time.Duration

	delaysMutex sync.RWMutex
}

type txConnectionKey struct {
	from types.Address
	to   types.Address
}

// NewFixedProcessingDelayModel creates a new fixed processing delay model with
// no initial delays.
func NewFixedProcessingDelayModel() *FixedProcessingDelayModel {
	return &FixedProcessingDelayModel{
		customTxDelays:          make(map[txConnectionKey]time.Duration),
		customFinalizationDelay: make(map[uint32]time.Duration),
	}
}

// --- Transaction Delay ---

// SetBaseTransactionDelay sets a delay applied to all transactions.
func (m *FixedProcessingDelayModel) SetBaseTransactionDelay(delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.baseTxDelay = delay
}

// SetConnectionTransactionDelay sets a custom delay for transactions from one
// address to another, overriding the base transaction delay.
func (m *FixedProcessingDelayModel) SetConnectionTransactionDelay(from, to types.Address, delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.customTxDelays[txConnectionKey{from: from, to: to}] = delay
}

// GetTransactionDelay returns the processing delay for a transaction.
func (m *FixedProcessingDelayModel) GetTransactionDelay(tx types.Transaction) time.Duration {
	m.delaysMutex.RLock()
	defer m.delaysMutex.RUnlock()

	if d, ok := m.customTxDelays[txConnectionKey{from: tx.From, to: tx.To}]; ok {
		return d
	}
	return m.baseTxDelay
}

// --- Block Finalization Delay ---

// SetBaseFinalizationDelay sets a delay applied to all block finalizations.
func (m *FixedProcessingDelayModel) SetBaseFinalizationDelay(delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.baseFinalizationDelay = delay
}

// SetCustomBlockFinalizationDelay sets a custom delay for finalizing a specific
// block number, overriding the base finalization delay.
func (m *FixedProcessingDelayModel) SetCustomBlockFinalizationDelay(blockNumber uint32, delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.customFinalizationDelay[blockNumber] = delay
}

// GetBlockFinalizationDelay returns the finalization delay for a block number.
func (m *FixedProcessingDelayModel) GetBlockFinalizationDelay(blockNumber uint32) time.Duration {
	m.delaysMutex.RLock()
	defer m.delaysMutex.RUnlock()

	if d, ok := m.customFinalizationDelay[blockNumber]; ok {
		return d
	}
	return m.baseFinalizationDelay
}
