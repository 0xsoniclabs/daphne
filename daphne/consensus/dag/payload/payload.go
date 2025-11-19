package payload

// Payload represents the data carried by a DAG vertex.
type Payload interface {
	// Size returns the size of the payload in bytes.
	Size() uint32
	// Clone creates a deep copy of the payload.
	Clone() Payload
}
