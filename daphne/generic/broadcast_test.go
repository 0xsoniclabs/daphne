package generic

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestBroadcastReceiver_WrapBroadcastReceiver_CallsWrappedFunction(t *testing.T) {
	called := false
	receiver := WrapBroadcastReceiver(func(message int) {
		require.Equal(t, 42, message)
		called = true
	})

	receiver.OnMessage(42)
	require.True(t, called, "wrapped function was not called")
}

func TestBroadcastReceivers_Deliver_DeliversCallbackToRegisteredReceivers(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiverA := NewMockBroadcastReceiver[int](ctrl)
	receiverB := NewMockBroadcastReceiver[int](ctrl)

	receivers := BroadcastReceivers[int]{}
	receivers.Register(receiverA)
	receivers.Register(receiverB)

	receiverA.EXPECT().OnMessage(42)
	receiverB.EXPECT().OnMessage(42)

	// Delivery runs asynchronously, so we need to wait for it to complete. The
	// synctest package provides a simple way to do this.
	synctest.Test(t, func(t *testing.T) {
		receivers.Deliver(42)
	})
}

func TestBroadcastReceivers_Register_AddsReceiverToRegistry(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiver := NewMockBroadcastReceiver[int](ctrl)

	receivers := BroadcastReceivers[int]{}
	require.Empty(t, receivers.receivers)

	receivers.Register(receiver)
	require.Len(t, receivers.receivers, 1)
	require.Equal(t, receiver, receivers.receivers[0])
}

func TestBroadcastReceivers_Register_MultipleRegistrationsAreIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiver := NewMockBroadcastReceiver[int](ctrl)

	receivers := BroadcastReceivers[int]{}
	require.Empty(t, receivers.receivers)

	receivers.Register(receiver)
	require.Len(t, receivers.receivers, 1)
	require.Equal(t, receiver, receivers.receivers[0])

	receivers.Register(receiver)
	require.Len(t, receivers.receivers, 1)
	require.Equal(t, receiver, receivers.receivers[0])
}

func TestBroadcastReceivers_Unregister_RemovesReceiverFromRegistry(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiverA := NewMockBroadcastReceiver[int](ctrl)
	receiverB := NewMockBroadcastReceiver[int](ctrl)

	receivers := BroadcastReceivers[int]{}
	receivers.Register(receiverA)
	receivers.Register(receiverB)
	require.Len(t, receivers.receivers, 2)

	receivers.Unregister(receiverA)
	require.Len(t, receivers.receivers, 1)
	require.Equal(t, receiverB, receivers.receivers[0])

	receivers.Unregister(receiverB)
	require.Empty(t, receivers.receivers)
}
