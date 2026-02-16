// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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
