package state

import (
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source state.go -destination=state_mock.go -package=state

// State of the blockchain, i.e. current block number and account balances.
type State interface {
	GetCurrentBlockNumber() uint32
	GetAccount(address types.Address) Account
	Apply(transactions []types.Transaction) types.Block
}

// Account represents a single account in the blockchain state.
type Account struct {
	Nonce   types.Nonce
	Balance types.Coin
}

func (a Account) String() string {
	return fmt.Sprintf("Nonce: %d, Balance: %d", a.Nonce, a.Balance)
}

// Genesis represents the initialization state of the blockchain.
type Genesis map[types.Address]Account

// state is the concrete implementation of the State interface.
type state struct {
	blockNumber uint32
	accounts    map[types.Address]Account
	delayModel  ProcessingDelayModel
	tracker     tracker.Tracker

	stateLock sync.Mutex
}

// NewState creates a new state instance initialized with the provided genesis
func NewState(g Genesis) *state {
	return NewStateBuilder().WithGenesis(g).Build()
}

// StateBuilder pattern for constructing state instances.
type StateBuilder struct {
	genesis    Genesis
	delayModel ProcessingDelayModel
	tracker    tracker.Tracker
}

// NewStateBuilder creates a new instance of StateBuilder with its default
// configuration.
func NewStateBuilder() *StateBuilder {
	return &StateBuilder{}
}

// WithGenesis sets the genesis state to be used by the state being built.
func (b *StateBuilder) WithGenesis(g Genesis) *StateBuilder {
	b.genesis = g
	return b
}

// WithDelayModel sets the processing delay model to be used by the state being
// built.
func (b *StateBuilder) WithDelayModel(model ProcessingDelayModel) *StateBuilder {
	b.delayModel = model
	return b
}

// WithTracker sets the tracker to be used by the state being built.
func (b *StateBuilder) WithTracker(tracker tracker.Tracker) *StateBuilder {
	b.tracker = tracker
	return b
}

// Build constructs the State instance from the builder's configuration.
func (b *StateBuilder) Build() *state {
	accounts := maps.Clone(b.genesis)
	if accounts == nil {
		accounts = map[types.Address]Account{}
	}
	s := &state{
		accounts:   accounts,
		delayModel: b.delayModel,
		tracker:    b.tracker,
	}
	return s
}

// GetCurrentBlockNumber returns the current block number of the state.
func (s *state) GetCurrentBlockNumber() uint32 {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	return s.blockNumber
}

// GetAccount retrieves the account information for the given address.
func (s *state) GetAccount(address types.Address) Account {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	return s.accounts[address]
}

// Apply processes a list of transactions, updates the state accordingly, and
// returns the resulting block.
func (s *state) Apply(transactions []types.Transaction) types.Block {
	// Track the confirmation of the incoming transactions.
	if s.tracker != nil {
		for _, tx := range transactions {
			s.tracker.Track(mark.TxConfirmed, "hash", tx.Hash(), "block", s.blockNumber)
		}
	}

	processed := []types.Transaction{}
	receipts := []types.Receipt{}
	for _, tx := range transactions {
		if receipt := s.process(tx, s.blockNumber); receipt != nil {
			processed = append(processed, tx)
			receipts = append(receipts, *receipt)
		}
	}

	// Finalization delay to simulate block finalization processing.
	if s.delayModel != nil {
		time.Sleep(s.delayModel.GetBlockFinalizationDelay(
			s.blockNumber,
			transactions,
		))
	}

	// Track the finalization of the processed transactions.
	if s.tracker != nil {
		for _, tx := range processed {
			s.tracker.Track(mark.TxFinalized, "hash", tx.Hash(), "block", s.blockNumber)
		}
	}

	res := types.Block{
		Number:       s.blockNumber,
		Transactions: processed,
		Receipts:     receipts,
	}

	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	s.blockNumber++
	return res

}

func (s *state) process(
	tx types.Transaction,
	blockNumber uint32,
) *types.Receipt {
	// Check whether the transaction can be processed.
	s.stateLock.Lock()
	account := s.accounts[tx.From]
	s.stateLock.Unlock()
	if account.Nonce != tx.Nonce {
		// Nonce mismatch causes the transaction to be skipped.
		if s.tracker != nil {
			s.tracker.Track(mark.TxSkipped, "hash", tx.Hash(), "block", blockNumber)
		}
		slog.Warn(
			"Transaction skipped due to nonce mismatch",
			"transaction", tx,
			"expectedNonce", account.Nonce,
		)
		return nil
	}

	// Track the processing of the transaction.
	if s.tracker != nil {
		s.tracker.Track(mark.TxBeginProcessing, "hash", tx.Hash(), "block", blockNumber)
		defer s.tracker.Track(mark.TxEndProcessing, "hash", tx.Hash(), "block", blockNumber)
	}

	// Apply transaction delay if configured.
	if s.delayModel != nil {
		time.Sleep(s.delayModel.GetTransactionDelay(tx))
	}

	// No matter the balance, nonce gets incremented.
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	account.Nonce++
	if account.Balance < tx.Value {
		// Transaction fails because there is not enough balance. However,
		// it becomes a part of this block.
		s.accounts[tx.From] = account
		return &types.Receipt{
			Success: false,
		}
	}

	// Transaction is successful, so we update the account balances.
	account.Balance -= tx.Value
	s.accounts[tx.From] = account

	toAccount := s.accounts[tx.To]
	toAccount.Balance += tx.Value
	s.accounts[tx.To] = toAccount

	return &types.Receipt{
		Success: true,
	}
}

func (s *state) String() string {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Blockchain State at Block %d:\n", s.blockNumber))
	for address, account := range s.accounts {
		sb.WriteString(fmt.Sprintf("Address: %s, %s\n", address, account))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}
