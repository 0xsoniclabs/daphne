package p2p

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFixedDelayModel_SetBaseDelay_CorrectlySetsBaseDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()

	delay := model.GetDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseDelay(100 * time.Millisecond)
	delay = model.GetDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseDelay(250 * time.Millisecond)
	delay = model.GetDelay("peer1", "peer2", Message{})
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedDelayModel_SetConnectionDelay_CorrectlySetsConnectionDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()
	model.SetBaseDelay(100 * time.Millisecond)

	delay := model.GetDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionDelay("peer1", "peer2", 150*time.Millisecond)
	delay = model.GetDelay("peer1", "peer2", Message{})
	require.Equal(150*time.Millisecond, delay)

	delay = model.GetDelay("peer2", "peer1", Message{})
	require.Equal(100*time.Millisecond, delay)
}
