package generic

// Broadcaster is a policy for broadcasting messages of type M to multiple receivers.
// No assumptions are made about the distribution policy - they could be best-effort
// only or a guaranteed delivery protocol, with or without timing guarantees.
type Broadcaster[M any] interface {
	Broadcast(message M)
	RegisterReceiver(receiver Receiver[M])
}

// Receiver is an interface that receives messages of type M from a Broadcaster and
// handles them.
type Receiver[M any] interface {
	OnMessage(message M)
}
