package generic

//go:generate mockgen -source broadcast.go -destination=broadcast_mock.go -package=generic

// Broadcaster is a policy for broadcasting messages of type M to multiple receivers.
// No assumptions are made about the distribution policy - they could be best-effort
// only or a guaranteed delivery protocol, with or without timing guarantees.
type Broadcaster[M any] interface {
	Broadcast(message M)
	RegisterReceiver(receiver BroadcastReceiver[M])
}

// BroadcastReceiver is an interface for receiving messages of type M from a
// broadcaster protocol.
type BroadcastReceiver[M any] interface {
	OnMessage(message M)
}
