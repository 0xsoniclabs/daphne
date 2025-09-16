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

func TestSampledDelayModel_SetBaseSendDistribution_SamplesDelaysCorrectly(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel(time.Millisecond)

	delay := model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	seed := int64(99)
	model.SetBaseSendDistribution(1.5, 0.4, &seed)

	delays := make([]time.Duration, 10000)
	for i := range 10000 {
		delays[i] = model.GetSendDelay("peer1", "peer2", Message{})
		require.Greater(delays[i], 0*time.Millisecond)
	}

	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}
	require.False(allSame, "Expected varying delays from log-normal distribution")
}

func TestSampledDelayModel_SetConnectionSendDistribution_SamplesDelaysCorrectly(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel(time.Millisecond)

	delay := model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	seed := int64(42)
	model.SetConnectionSendDistribution("peer1", "peer2", 2.0, 0.5, &seed)

	delays := make([]time.Duration, 10000)
	for i := range 10000 {
		delays[i] = model.GetSendDelay("peer1", "peer2", Message{})
		require.Greater(delays[i], 0*time.Millisecond)
	}

	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}
	require.False(allSame, "Expected varying delays from log-normal distribution")
}

func TestSampledDelayModel_SetConnectionSendDistribution_OverridesBaseDistribution(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel(time.Millisecond)

	baseSeed := int64(42)
	model.SetBaseSendDistribution(1.0, 0.3, &baseSeed)

	connSeed := int64(123)
	model.SetConnectionSendDistribution("peer1", "peer2", 3.0, 0.2, &connSeed)

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		customDelay := model.GetSendDelay("peer1", "peer2", Message{})
		baseDelay := model.GetSendDelay("peer2", "peer1", Message{})
		require.Greater(customDelay, 0*time.Millisecond)
		require.Greater(baseDelay, 0*time.Millisecond)
		require.Greater(customDelay, baseDelay, "Expected custom delivery delays to be larger than base delays")
	}
}

func TestSampledDelayModel_SetBaseDeliveryDistribution_SamplesDelaysCorrectly(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel(time.Millisecond)

	delay := model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	seed := int64(42)
	model.SetBaseDeliveryDistribution(2.0, 0.5, &seed)

	delays := make([]time.Duration, 10000)
	for i := range 10000 {
		delays[i] = model.GetDeliveryDelay("peer1", "peer2", Message{})
		require.Greater(delays[i], 0*time.Millisecond)
	}

	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}
	require.False(allSame, "Expected varying delays from log-normal distribution")
}

func TestSampledDelayModel_SetConnectionDeliveryDistribution_SamplesDelaysCorrectly(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel(time.Millisecond)

	delay := model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	seed := int64(42)
	model.SetConnectionDeliveryDistribution("peer1", "peer2", 1.5, 0.4, &seed)

	delays := make([]time.Duration, 10000)
	for i := range 10000 {
		delays[i] = model.GetDeliveryDelay("peer1", "peer2", Message{})
		require.Greater(delays[i], 0*time.Millisecond)
	}

	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}
	require.False(allSame, "Expected varying delays from log-normal distribution")
}

func TestSampledDelayModel_SetConnectionDeliveryDistribution_OverridesBaseDistribution(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel(time.Millisecond)

	baseSeed := int64(42)
	model.SetBaseDeliveryDistribution(1.0, 0.3, &baseSeed)

	connSeed := int64(123)
	model.SetConnectionDeliveryDistribution("peer1", "peer2", 3.0, 0.2, &connSeed)

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		customDelay := model.GetDeliveryDelay("peer1", "peer2", Message{})
		baseDelay := model.GetDeliveryDelay("peer2", "peer1", Message{})
		require.Greater(customDelay, 0*time.Millisecond)
		require.Greater(baseDelay, 0*time.Millisecond)
		require.Greater(customDelay, baseDelay, "Expected custom delivery delays to be larger than base delays")
	}
}
