package db

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const testEpoch = 8000
const testDBPath = "sample.db"

func Test_GetLeaders_ReturnsExpectedValuesForSampleDB(t *testing.T) {
	reader, err := NewEventDBReader(testDBPath)
	require.NoError(t, err)

	leaders, err := reader.GetLeaders(testEpoch)
	require.NoError(t, err)

	require.Equal(t, []consensus.ValidatorId{7, 36}, leaders)
}

func TestEventDBReader_GetEpochRange_ReturnsExpectedValuesForSampleDB(t *testing.T) {
	reader, err := NewEventDBReader(testDBPath)
	require.NoError(t, err)

	epochMin, epochMax, err := reader.GetEpochRange()
	require.NoError(t, err)

	require.Equal(t, 8000, epochMin)
	require.Equal(t, 8001, epochMax)
}

func TestEventDBReader_GetValidators_ReturnsExpectedValuesForSampleDB(t *testing.T) {
	reader, err := NewEventDBReader(testDBPath)
	require.NoError(t, err)

	validators, weights, err := reader.GetValidators(testEpoch)
	require.NoError(t, err)

	require.Equal(t, []consensus.ValidatorId{1, 2, 3}, validators)
	require.Equal(t, []uint32{15, 20, 30}, weights)
}

func TestEventDBReader_GetEvents_ReturnsExpectedValuesForSampleDB(t *testing.T) {
	require := require.New(t)
	reader, err := NewEventDBReader(testDBPath)
	require.NoError(err)

	expectedEvents := []*DBEvent{}
	validators := []consensus.ValidatorId{7, 22, 7, 36, 21}
	for i := range 5 {
		expectedEvents = append(
			expectedEvents,
			&DBEvent{
				ValidatorId: validators[i],
				Seq:         uint32(i + 1),
				Frame:       i + 1,
			},
		)
	}
	// Test the self-parent prioritization on event e2, putting
	// e0 as it's self-parent. Make e0 and e2 have the same validator.
	expectedEvents[2].ValidatorId = expectedEvents[0].ValidatorId

	// e0 -> (e1 -> e2, (e3, e4)), e2
	expectedEvents[0].Parents = []*DBEvent{}
	expectedEvents[1].Parents = []*DBEvent{expectedEvents[0]}
	expectedEvents[2].Parents = []*DBEvent{expectedEvents[0], expectedEvents[1]}
	expectedEvents[3].Parents = []*DBEvent{expectedEvents[1]}
	expectedEvents[4].Parents = []*DBEvent{expectedEvents[1]}

	events, err := reader.GetEvents(testEpoch)
	require.NoError(err)

	require.Equal(expectedEvents, events)
}

func TestEventDBReader_NewEventDBReader_ErrorOnInvalidPath(t *testing.T) {
	// The only way for [sql.Open] to return an error is to provide an invalid driver name.
	// Even with a non-existent file path, it will not result in an error as
	// the method only prepares a connection pool without actually connecting to the database.
	_, err := newEventDBReader("sqldaphne3", testDBPath)
	require.Error(t, err)
}

func TestEventDBReader_GetX_ReturnsErrorOnQueryFail(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	conn := NewMockDBConnection(ctrl)

	dbReader := &EventDBReader{conn: conn}

	conn.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("query error")).AnyTimes()

	_, err := dbReader.GetLeaders(0)
	require.ErrorContains(err, "query")

	_, _, err = dbReader.GetEpochRange()
	require.ErrorContains(err, "query")

	_, _, err = dbReader.GetValidators(0)
	require.ErrorContains(err, "query")

	_, err = dbReader.GetEvents(0)
	require.ErrorContains(err, "query")

	err = dbReader.appointParents(map[string]*DBEvent{}, 0)
	require.ErrorContains(err, "query")
}

func TestEventDBReader_GetX_ReturnsErrorOnScanFail(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	conn := NewMockDBConnection(ctrl)
	rows := NewMockDBRows(ctrl)

	dbReader := &EventDBReader{conn: conn}
	conn.EXPECT().Query(gomock.Any(), gomock.Any()).Return(rows, nil).AnyTimes()

	rows.EXPECT().Next().Return(true).AnyTimes()
	rows.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("scan error")).AnyTimes()
	rows.EXPECT().Close().Return(nil).AnyTimes()

	_, err := dbReader.GetLeaders(1)
	require.ErrorContains(err, "scan")
	_, _, err = dbReader.GetEpochRange()
	require.ErrorContains(err, "scan")
	_, _, err = dbReader.GetValidators(1)
	require.ErrorContains(err, "scan")
	_, err = dbReader.GetEvents(1)
	require.ErrorContains(err, "scan")
	err = dbReader.appointParents(map[string]*DBEvent{}, 1)
	require.ErrorContains(err, "scan")
}

func TestEventDBReader_GetEpochRange_ErrorOnNoRows(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	conn := NewMockDBConnection(ctrl)
	rows := NewMockDBRows(ctrl)

	dbReader := &EventDBReader{conn: conn}

	conn.EXPECT().Query(gomock.Any(), gomock.Any()).Return(rows, nil)
	rows.EXPECT().Next().Return(false)
	rows.EXPECT().Close().Return(nil)

	_, _, err := dbReader.GetEpochRange()
	require.ErrorContains(err, "no non-empty epochs")
}

func TestEventDBReader_appointParents_ErrorOnUnknownEvents(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	conn := NewMockDBConnection(ctrl)
	rows := NewMockDBRows(ctrl)

	dbReader := &EventDBReader{conn: conn}

	conn.EXPECT().Query(gomock.Any(), gomock.Any()).Return(rows, nil).Times(2)
	rows.EXPECT().Next().Return(true).Times(2)
	rows.EXPECT().Close().Return(nil).Times(2)

	// child event not found
	rows.EXPECT().Scan(gomock.Any()).Do(
		func(eventIds ...*string) {
			*eventIds[0] = "eX"
		})
	err := dbReader.appointParents(map[string]*DBEvent{"e1": {}}, 1)
	require.ErrorContains(err, "incomplete event db - child event not found")

	// parent event not found
	rows.EXPECT().Scan(gomock.Any()).Do(
		func(eventIds ...*string) {
			*eventIds[0] = "e1"
			*eventIds[1] = "eX"
		})
	err = dbReader.appointParents(map[string]*DBEvent{"e1": {}}, 1)
	require.ErrorContains(err, "incomplete event db - parent event not found")
}

func TestEventDBReader_closeRowsAndCombineErrors_ChainsErrorsCorrectly(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	rows := NewMockDBRows(ctrl)

	var err = fmt.Errorf("initial error")

	rows.EXPECT().Close().Return(fmt.Errorf("close error"))

	closeRowsAndCombineErrors(&err, rows)
	require.ErrorContains(err, "initial error")
	require.ErrorContains(err, "close error")
}
