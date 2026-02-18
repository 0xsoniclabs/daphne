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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventMatcher_WithEventId_MatchesWithAnotherEventId(t *testing.T) {
	id := EventId{1}
	matcher := WithEventId(id)

	require.True(t, matcher.Matches(&Event{id: id}))
}

func TestEventMatcher_WithEventId_DoesNotMatchWithAnotherEventId(t *testing.T) {
	matcher := WithEventId(EventId{1})

	require.False(t, matcher.Matches(&Event{id: EventId{2}}))
}

func TestEventMatcher_WithEventId_DoesNotMatchWithAnotherNonEvent(t *testing.T) {
	matcher := WithEventId(EventId{1})

	require.False(t, matcher.Matches(struct{}{}))
}

func TestEventMatcher_String_PrintsMessageStringPayload(t *testing.T) {
	matcher := WithEventId(EventId{1, 15, 16})

	require.Equal(t, "Event with ID: is equal to 010F100000000000 (model.EventId)", matcher.String())
}
