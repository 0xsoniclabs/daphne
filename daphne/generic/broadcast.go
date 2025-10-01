package generic

//go:generate mockgen -source broadcast.go -destination=broadcast_mock.go -package=generic

// Broadcaster is a policy for broadcasting messages of type M to multiple receivers.
// No assumptions are made about the distribution policy - they could be best-effort
// only or a guaranteed delivery protocol, with or without timing guarantees.
type Broadcaster[M any] interface {
	Broadcast(message M)
	RegisterReceiver(receiver BroadcastReceiver[M])
	UnregisterReceiver(receiver BroadcastReceiver[M])
}

// BroadcastReceiver is an interface for receiving messages of type M from a
// broadcaster protocol. Callbacks are guaranteed to be called asynchronously
// to the `Broadcast` calls, but no assumptions are made about the timing or
// ordering of the messages.
type BroadcastReceiver[M any] interface {
	OnMessage(message M)
}

// --- Adapter for functions to BroadcastReceiver ---

// WrapBroadcastReceiver wraps a function with the signature
// func(message M) into a BroadcastReceiver adapter that can be registered
// on a [Broadcaster]. This is a convenience function to allow using simple
// functions as message handlers without having to define a new type.
func WrapBroadcastReceiver[M any](f func(message M)) BroadcastReceiver[M] {
	return &broadcastReceiverAdapter[M]{f: f}
}

type broadcastReceiverAdapter[M any] struct {
	f func(message M)
}

func (h *broadcastReceiverAdapter[M]) OnMessage(message M) {
	h.f(message)
}
