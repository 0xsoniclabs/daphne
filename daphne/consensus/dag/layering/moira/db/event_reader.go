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

package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/consensus"

	_ "github.com/mattn/go-sqlite3"
)

//go:generate mockgen -source event_reader.go -destination=event_reader_mock.go -package=db

// DBEvent represents an event read from the Sonic event database.
// It includes the validator ID, sequence number, frame, and parent events, of
// an event created and processed by the Sonic Lachesis implementation.
// The Seq and Frame can be used to verify the Sequence Number and Frame assigned
// by the Daphne Lachesis implementation.
type DBEvent struct {
	ValidatorId consensus.ValidatorId
	Seq         uint32
	Frame       int
	Parents     []*DBEvent
}

// EventDBReader provides methods to read events and related data from a Sonic
// event database.
type EventDBReader struct {
	conn dbConnection
}

// NewEventDBReader creates a new EventDBReader connected to the SQLite3
// database located at the given path.
func NewEventDBReader(dbPath string) (*EventDBReader, error) {
	return newEventDBReader("sqlite3", dbPath)
}

func newEventDBReader(driverName string, dbPath string) (*EventDBReader, error) {
	dbConn, err := sql.Open(driverName, dbPath)
	if err != nil {
		return nil, err
	}
	return &EventDBReader{conn: &dbConnectionAdapter{DB: dbConn}}, nil
}

// dbConnection is an interface abstracting a database connection for querying.
// This allows for easier testing and mocking of database interactions.
type dbConnection interface {
	Query(query string, args ...any) (dbRows, error)
}

// dbRows is an interface abstracting the result set returned by a database query,
// allowing for the row interactions to be mocked.
type dbRows interface {
	Scan(dest ...any) error
	Next() bool
	Close() error
}

// dbConnectionAdapter adapts a *sql.DB to the dbConnection interface.
// This is done through the [dbConnectionAdapter.Query] method by generalizing
// the return type to the [dbRows] interface.
type dbConnectionAdapter struct {
	*sql.DB
}

func (a *dbConnectionAdapter) Query(query string, args ...any) (dbRows, error) {
	return a.DB.Query(query, args...)
}

// GetEpochRange retrieves the minimum and maximum epoch IDs present in the
// event database. It returns an error if no epochs with events are found.
//
// Note:
// It queries the `Event` table as `Validator` table may include (empty) epochs
// without events (known bug in the events.db generation process).
func (r *EventDBReader) GetEpochRange() (_ int, _ int, err error) {
	rows, err := r.conn.Query(`
		SELECT MIN(e.EpochId), MAX(e.EpochId)
		FROM Event e
	`)
	if err != nil {
		return 0, 0, err
	}
	defer closeRowsAndCombineErrors(&err, rows)

	var epochMin, epochMax int
	if !rows.Next() {
		return 0, 0, fmt.Errorf("no non-empty epochs in database")
	}
	err = rows.Scan(&epochMin, &epochMax)
	if err != nil {
		return 0, 0, err
	}
	return epochMin, epochMax, nil
}

// GetValidators retrieves the list of validator IDs and their corresponding
// weights for the specified epoch from the event database.
func (r *EventDBReader) GetValidators(epoch int) (_ []consensus.ValidatorId, _ []uint32, err error) {
	rows, err := r.conn.Query(`
		SELECT ValidatorId, Weight
		FROM Validator
		WHERE EpochId = ?
	`, epoch)
	if err != nil {
		return nil, nil, err
	}
	defer closeRowsAndCombineErrors(&err, rows)

	validators := make([]consensus.ValidatorId, 0)
	weights := make([]uint32, 0)
	for rows.Next() {
		var validatorId consensus.ValidatorId
		var weight uint32

		err = rows.Scan(&validatorId, &weight)
		if err != nil {
			return nil, nil, err
		}

		validators = append(validators, validatorId)
		weights = append(weights, weight)
	}
	return validators, weights, nil
}

// GetEvents retrieves all events for the specified epoch from the event
// database. It returns the events in the order of their Lamport numbers -
// ready for no-delay processing (note that this ordering is not unique).
// It also populates the parent events for each event.
func (r *EventDBReader) GetEvents(epoch int) (_ []*DBEvent, err error) {
	rows, err := r.conn.Query(`
		SELECT e.EventHash, e.ValidatorId, e.SequenceNumber, e.FrameId
		FROM Event e
		WHERE e.EpochId = ?
		ORDER BY e.LamportNumber ASC
	`, epoch)
	if err != nil {
		return nil, err
	}
	defer closeRowsAndCombineErrors(&err, rows)

	// Use original event hashes as keys to later assign parents.
	eventMap := make(map[string]*DBEvent)
	eventsOrdered := make([]*DBEvent, 0)

	for rows.Next() {
		var hashStr string
		var validatorId consensus.ValidatorId
		var seq uint32
		var frame int
		err = rows.Scan(&hashStr, &validatorId, &seq, &frame)
		if err != nil {
			return nil, err
		}

		event := &DBEvent{
			ValidatorId: validatorId,
			Seq:         seq,
			Frame:       frame,
			Parents:     make([]*DBEvent, 0),
		}
		eventsOrdered = append(eventsOrdered, event)
		eventMap[hashStr] = event
	}
	return eventsOrdered, r.appointParents(eventMap, epoch)
}

// GetLeaders retrieves the list of leader validator IDs for the specified
// epoch from the event database. The leaders are returned in the order of
// their elected frames, where the first leader corresponds to frame 1.
func (r *EventDBReader) GetLeaders(epoch int) (_ []consensus.ValidatorId, err error) {
	rows, err := r.conn.Query(`
		SELECT e.ValidatorId
		FROM Atropos a JOIN Event e ON a.AtroposId = e.EventId
		WHERE e.EpochId = ?
		ORDER BY a.AtroposId ASC
	`, epoch)
	if err != nil {
		return nil, err
	}
	defer closeRowsAndCombineErrors(&err, rows)

	leaders := make([]consensus.ValidatorId, 0)
	for rows.Next() {
		var validatorId consensus.ValidatorId
		err = rows.Scan(&validatorId)
		if err != nil {
			return nil, err
		}

		leaders = append(leaders, validatorId)
	}
	return leaders, nil
}

// appointParents populates the Parents field of each DBEvent in the provided
// eventMap.
func (r *EventDBReader) appointParents(eventMap map[string]*DBEvent, epoch int) (err error) {
	rows, err := r.conn.Query(`
		SELECT e.EventHash, eParent.EventHash
		FROM Event e JOIN Parent p ON e.EventId = p.EventId JOIN Event eParent ON eParent.EventId = p.ParentId
		WHERE e.EpochId = ?
	`, epoch)
	if err != nil {
		return err
	}
	defer closeRowsAndCombineErrors(&err, rows)

	for rows.Next() {
		var eventHashStr string
		var parentHashStr string
		err = rows.Scan(&eventHashStr, &parentHashStr)
		if err != nil {
			return err
		}

		event, ok := eventMap[eventHashStr]
		if !ok {
			return fmt.Errorf(
				"incomplete event db - child event not found. epoch: %d, child event: %s, parent event: %s",
				epoch,
				eventHashStr,
				parentHashStr,
			)
		}
		if _, ok := eventMap[parentHashStr]; !ok {
			return fmt.Errorf(
				"incomplete event db - parent event not found. epoch: %d, child event: %s, parent event: %s",
				epoch,
				eventHashStr,
				parentHashStr,
			)
		}
		event.Parents = append(event.Parents, eventMap[parentHashStr])
		// ensure the self parent is first in the slice
		if eventMap[parentHashStr].ValidatorId == event.ValidatorId {
			event.Parents[0], event.Parents[len(event.Parents)-1] = event.Parents[len(event.Parents)-1], event.Parents[0]
		}
	}
	return nil
}

func closeRowsAndCombineErrors(errPtr *error, rows dbRows) {
	if err := rows.Close(); err != nil {
		*errPtr = errors.Join(*errPtr, err)
	}
}
