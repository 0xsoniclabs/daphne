package state

import (
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

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
}

// NewState creates a new state instance initialized with the provided genesis
func NewState(g Genesis) *state {
	return NewStateBuilder().WithGenesis(g).Build()
}

// StateBuilder pattern for constructing state instances.
type StateBuilder struct {
	genesis    Genesis
	delayModel ProcessingDelayModel
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

// Build constructs the State instance from the builder's configuration.
func (b *StateBuilder) Build() *state {
	s := &state{
		accounts:   maps.Clone(b.genesis),
		delayModel: b.delayModel,
	}
	return s
}

// GetCurrentBlockNumber returns the current block number of the state.
func (s *state) GetCurrentBlockNumber() uint32 {
	return s.blockNumber
}

// GetAccount retrieves the account information for the given address.
func (s *state) GetAccount(address types.Address) Account {
	return s.accounts[address]
}

// Apply processes a list of transactions, updates the state accordingly, and
// returns the resulting block.
func (s *state) Apply(transactions []types.Transaction) types.Block {
	processed := []types.Transaction{}
	receipts := []types.Receipt{}
	for _, tx := range transactions {
		account := s.accounts[tx.From]
		if account.Nonce != tx.Nonce {
			// Nonce mismatch causes the transaction to be skipped.
			slog.Warn(
				"Transaction skipped due to nonce mismatch",
				"transaction", tx,
				"expectedNonce", account.Nonce,
			)
			continue
		}

		// Apply transaction delay if configured.
		if s.delayModel != nil {
			time.Sleep(s.delayModel.GetTransactionDelay(tx))
		}
		processed = append(processed, tx)

		// No matter the balance, nonce gets incremented.
		account.Nonce = tx.Nonce
		if account.Balance < tx.Value {
			// Transaction fails because there is not enough balance. However,
			// it becomes a part of this block.
			s.accounts[tx.From] = account
			receipts = append(receipts, types.Receipt{
				Success: false,
			})
			continue
		}

		// Transaction is successful, so we update the account balances.
		account.Balance -= tx.Value
		s.accounts[tx.From] = account

		toAccount := s.accounts[tx.To]
		toAccount.Balance += tx.Value
		s.accounts[tx.To] = toAccount

		receipts = append(receipts, types.Receipt{
			Success: true,
		})
	}

	// Finalization delay to simulate block finalization processing.
	if s.delayModel != nil {
		time.Sleep(s.delayModel.GetBlockFinalizationDelay(
			s.blockNumber,
			transactions,
		))
	}

	s.blockNumber++
	return types.Block{
		Number:       s.blockNumber,
		Transactions: processed,
		Receipts:     receipts,
	}
}

func (s *state) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Blockchain State at Block %d:\n", s.blockNumber))
	for address, account := range s.accounts {
		sb.WriteString(fmt.Sprintf("Address: %s, %s\n", address, account))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}
