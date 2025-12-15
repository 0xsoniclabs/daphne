// Package single_proposer implements a DAG payload strategy where a single
// proposer is responsible for proposing blocks in the consensus protocol.
// It models the single-proposer mode implemented in the Sonic network.
//
// The single proposer protocol was proposed as a respond to new challenges in
// the block formation process due to EIP-7702. A [design doc] outlining the
// motivations and design considerations for the single proposer protocol can be
// found here. A [report] detailing the implementation details is also available.
//
// [design doc]: https://docs.google.com/document/d/1jiz-n1nNCAM2r0HnEXhsj9LJfdv_AxpNWhlM0pdo17Q/edit?tab=t.0
// [report]: https://docs.google.com/document/d/1m2-f-3ddFtsxNoAva96gbbHnbQ8P5PnYV532v1DWXzA/edit?tab=t.0
package single_proposer
