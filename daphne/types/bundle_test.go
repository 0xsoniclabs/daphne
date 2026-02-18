// Copyright 2026 Sonic Operations Ltd
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
)

func TestBundle_String_PrintBundleContents(t *testing.T) {
	tests := map[*Bundle]string{
		{
			Number:       0,
			Transactions: []Transaction{},
		}: "Bundle{Number:0 Transactions:[] Timestamp:0001-01-01 00:00:00 +0000 UTC}",
		{
			Number: 1,
			Transactions: []Transaction{
				{From: 0, To: 0, Value: 0, Nonce: 0},
				{From: 0, To: 1, Value: 2, Nonce: 3},
			},
		}: "Bundle{Number:1 Transactions:[{From:#0 To:#0 Value:$0 Nonce:0} {From:#0 To:#1 Value:$2 Nonce:3}] Timestamp:0001-01-01 00:00:00 +0000 UTC}",
	}

	for bundle, expected := range tests {
		t.Run(fmt.Sprintf("%d", bundle.Number), func(t *testing.T) {
			if bundle.String() != expected {
				t.Errorf("expected %s but got %s", expected, bundle.String())
			}
		})
	}
}
