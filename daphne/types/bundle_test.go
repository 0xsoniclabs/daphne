package types

import (
	"fmt"
	"testing"
)

func TestBundle_String_PrintBundleContents(t *testing.T) {
	tests := map[*Bundle]string{
		{
			Number:       0,
			Transactions: []Transaction{},
		}: "Bundle{Number:0 Transactions:[]}",
		{
			Number: 1,
			Transactions: []Transaction{
				{From: 0, To: 0, Value: 0, Nonce: 0},
				{From: 0, To: 1, Value: 2, Nonce: 3},
			},
		}: "Bundle{Number:1 Transactions:[{From:#0 To:#0 Value:$0 Nonce:0} {From:#0 To:#1 Value:$2 Nonce:3}]}",
	}

	for bundle, expected := range tests {
		t.Run(fmt.Sprintf("%d", bundle.Number), func(t *testing.T) {
			if bundle.String() != expected {
				t.Errorf("expected %s but got %s", expected, bundle.String())
			}
		})
	}
}
