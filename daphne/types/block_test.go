package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlock_String_PrintBlockContents(t *testing.T) {
	tests := map[*Block]string{
		{
			Number:       0,
			Transactions: []Transaction{},
			Receipts:     []Receipt{},
		}: "Block{Number:0 Transactions:[] Receipts:[]}",
		{
			Number: 1,
			Transactions: []Transaction{
				{},
				{0, 1, 2, 3},
			},
			Receipts: []Receipt{
				{Success: true},
				{Success: false},
			},
		}: "Block{Number:1 Transactions:[{From:#0 To:#0 Value:$0 Nonce:0}" +
			" {From:#0 To:#1 Value:$2 Nonce:3}]" +
			" Receipts:[{Success:true} {Success:false}]}",
	}

	for block, expected := range tests {
		t.Run(fmt.Sprintf("%d", block.Number), func(t *testing.T) {
			require.Equal(t, expected, block.String())
		})
	}
}
