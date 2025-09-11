package p2p

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFixedDelayModel_SetBaseDeliveryDelay_CorrectlySetsBaseDeliveryDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()

	delay := model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseDeliveryDelay(100 * time.Millisecond)
	delay = model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseDeliveryDelay(250 * time.Millisecond)
	delay = model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedDelayModel_SetConnectionDeliveryDelay_CorrectlySetsConnectionDeliveryDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()
	model.SetBaseDeliveryDelay(100 * time.Millisecond)

	delay := model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionDeliveryDelay("peer1", "peer2", 150*time.Millisecond)
	delay = model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(150*time.Millisecond, delay)

	delay = model.GetDeliveryDelay("peer2", "peer1", Message{})
	require.Equal(100*time.Millisecond, delay)
}

func TestFixedDelayModel_SetBaseSendDelay_CorrectlySetsBaseSendDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()

	delay := model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseSendDelay(100 * time.Millisecond)
	delay = model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseSendDelay(250 * time.Millisecond)
	delay = model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedDelayModel_SetConnectionSendDelay_CorrectlySetsConnectionSendDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()
	model.SetBaseSendDelay(100 * time.Millisecond)

	delay := model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionSendDelay("peer1", "peer2", 150*time.Millisecond)
	delay = model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(150*time.Millisecond, delay)

	delay = model.GetSendDelay("peer2", "peer1", Message{})
	require.Equal(100*time.Millisecond, delay)
}
