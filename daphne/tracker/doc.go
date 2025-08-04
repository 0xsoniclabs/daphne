// Package tracker provides a mechanism to track activities in Daphne.
//
// At its core, the package provides a Tracker which allows users to report
// the occurrence of events with associated metadata. For instance,
//
//   tracker.Track("TxReceived", "hash", tx.Hash())
//
// could be used to track the reception of a transaction with its hash being
// stored as metadata. At a different point, an event like
//
//  tracker.Track("TxProcessed", "hash", tx.Hash(), "status", "success")
//
// could be used to track the processing of a transaction. The key "hash" can
// then be used to correlate these events.
//
// To start tracking events, a root Tracker instance can be created using
// [New]. This root Tracker can be used to track events directly or to create
// sub-Trackers that add additional metadata to the events.
//
// For instance, the following code creates a root Tracker and sub-trackers for
// for various nodes in the system:
//
//   root := tracker.New()
//   sub1 := root.With("node", p2p.PeerId("node1"))
//   sub2 := root.With("node", p2p.PeerId("node2"))
//
// These sub-Trackers can be used like regular Trackers, but they will add the
// "node" metadata to all events tracked by them. This allows for a hierarchical
// tracking structure, where each component can add its own context to the
// events.
//
// All tracked events end up in the root Tracker, where they can be obtained
// using [GetAll]. The events are stored as [Entry] instances, which contain
// the event name, the time of the event, and a [Metadata] instance with the
// associated metadata.

package tracker
