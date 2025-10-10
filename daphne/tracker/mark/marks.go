package mark

//go:generate stringer -type=Mark -output marks_string.go

// Mark is a unique identifier for marking events to be tracked by the tracker.
// Marks are used to identify specific events in the system, allowing for
// tracking and processing of those events for post-mortem analysis.
//
// Marks for various data or control flows through the system should be defined
// in this file, combined with a description of their semantic interpretation.
// The names of those marks are then used when exporting events collected by the
// tracker for external processing.
type Mark int

const (
	// --- Network Messages ---

	// Marks for tracking the propagation of messages throughout the network.
	// Each of these marks should be tracked with the following metadata:
	// - id: a unique message ID (as an integer)
	// - type: the type of message (as a string)
	// - from: the sender of the message
	// - to: the receiver of the message
	MsgSent     Mark = iota // A message got sent by a peer to another
	MsgReceived             // A message got received by a peer
	MsgConsumed             // A message was processed by the receiver side

	// --- Transactions ---

	// Marks for tracking the lifecycle of transactions within the system.
	// Each of these marks should be tracked with the following metadata:
	// - hash: the hash of the transaction as a universal identifier
	// - node: the node on which the mark has been recorded
	// - block: the block number to which the event is associated (if applicable)
	TxSubmitted       // A transaction got submitted to an RPC endpoint
	TxAddedToPool     // A transaction got added to the pool
	TxConfirmed       // A transaction was confirmed to be part of a block
	TxBeginProcessing // A transaction is about to be processed
	TxEndProcessing   // A transaction has finished processing
	TxSkipped         // A transaction was skipped by the processor
	TxFinalized       // A transaction's receipt is ready
)
