package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReceipt_String_PrintWithSuccessStatus(t *testing.T) {
	tests := map[*Receipt]string{
		{true}:  "{Success:true}",
		{false}: "{Success:false}",
	}

	for receipt, expected := range tests {
		t.Run(fmt.Sprintf("%t", receipt.Success), func(t *testing.T) {
			require.Equal(t, expected, receipt.String())
		})
	}
}
