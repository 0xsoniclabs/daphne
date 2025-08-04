package analysis

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestAnalysis_ProduceReport(t *testing.T) {
	const numEntries = 100_000

	// Produce some example data
	tracker := tracker.New()

	transactions := []types.Transaction{}
	for i := range numEntries {
		tx := types.Transaction{
			From:  0,
			To:    1,
			Nonce: types.Nonce(i),
		}
		transactions = append(transactions, tx)
		tracker.Track(TxCreated, "hash", tx.Hash())
	}

	for _, tx := range transactions {
		tracker.Track(TxProcessed, "hash", tx.Hash())
	}

	// Produce the report
	location, err := ProduceReport(tracker.GetAll(), ".")
	require.NoError(t, err)
	location, err = filepath.Abs(location)
	require.NoError(t, err)
	fmt.Printf("Report generated at: %s\n", location)
	//t.Fail()
}
