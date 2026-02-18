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

package model

import (
	"fmt"

	"go.uber.org/mock/gomock"
)

// WithEventId is a gomock matcher for [*Event] instances that checks whether a
// given instance has a specified id. The id can be any type that implements the
// gomock.Matcher interface, or a specific value that should be matched exactly.
func WithEventId(id any) gomock.Matcher {
	if matcher, ok := id.(gomock.Matcher); ok {
		return withEventId{id: matcher}
	}
	return WithEventId(gomock.Eq(id))
}

type withEventId struct {
	id gomock.Matcher
}

func (i withEventId) Matches(arg any) bool {
	event, ok := arg.(*Event)
	if !ok {
		return false
	}
	return i.id.Matches(event.EventId())
}

func (i withEventId) String() string {
	return fmt.Sprintf("Event with ID: %s", i.id.String())
}
