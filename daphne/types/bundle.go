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

package types

import (
	"fmt"
	"time"
)

// Bundle represents a verifiable product of consensus.
// It contains a bundle number and a slice of transactions.
type Bundle struct {
	Number       uint32
	Transactions []Transaction
	Timestamp    time.Time
}

func (b *Bundle) String() string {
	return fmt.Sprintf("Bundle%+v", *b)
}
