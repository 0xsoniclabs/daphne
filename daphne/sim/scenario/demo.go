package scenario

import (
	"fmt"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/node"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// DemoScenario is a simple scenario starting a fixed number of nodes in a
// network with a configurable topology to which transactions are sent at a
// fixed rate.
type DemoScenario struct {
	NumNodes                  int
	TxPerSecond               int
	Duration                  time.Duration
	Broadcast                 broadcast.Protocol
	Consensus                 consensus.Factory
	Topology                  p2p.NetworkTopology
	NetworkLatencyModel       p2p.LatencyModel
	StateProcessingDelayModel state.ProcessingDelayModel

	nodeNameGenerator    func(int) string
	transactionGenerator func(int) types.Transaction
}

func (d *DemoScenario) Run(
	log Logger,
	tracker tracker.Tracker,
) error {
	// Handle default parameters.
	numNodes := d.NumNodes
	if numNodes <= 0 {
		numNodes = 3
	}
	txPerSecond := d.TxPerSecond
	if txPerSecond <= 0 {
		txPerSecond = 100
	}
	duration := d.Duration
	if duration <= 0 {
		duration = 5 * time.Second
	}

	committee := *consensus.NewUniformCommittee(numNodes)

	consensusFactory := d.Consensus
	if consensusFactory == nil {
		consensusFactory = central.Factory{
			BroadcastFactory: broadcast.GetFactory[uint32, central.BundleMessage](
				d.Broadcast,
			),
		}
	}

	getNodeName := d.nodeNameGenerator
	if getNodeName == nil {
		getNodeName = func(i int) string {
			return fmt.Sprintf("N-%03d", i+1)
		}
	}

	getTx := d.transactionGenerator
	if getTx == nil {
		getTx = func(i int) types.Transaction {
			return types.Transaction{From: 0, To: 1, Nonce: types.Nonce(i)}
		}
	}

	tracker = tracker.With("numNodes", numNodes)

	log.Info(
		"Starting Demo Scenario",
		"numNodes", numNodes,
		"txPerSecond", txPerSecond,
		"duration", duration,
	)

	// Step 1: define genesis data for the demo
	genesis := state.Genesis{}

	// Step 2: set up a network of nodes.
	log.Info("Setting up network", "numNodes", numNodes)
	builder := p2p.NewNetworkBuilder().WithTracker(tracker)

	// Use provided topology, or default to fully-meshed for backward
	// compatibility.
	if d.Topology != nil {
		builder = builder.WithTopology(d.Topology)
	}

	// Use provided network latency model if specified.
	if d.NetworkLatencyModel != nil {
		builder = builder.WithLatency(d.NetworkLatencyModel)
	}

	network := builder.Build()

	config := node.NodeConfig{
		Network:                   network,
		Broadcast:                 d.Broadcast,
		Consensus:                 consensusFactory,
		Genesis:                   genesis,
		Tracker:                   tracker,
		StateProcessingDelayModel: d.StateProcessingDelayModel,
	}

	validators := committee.Validators()
	nodes := make([]*node.Node, numNodes)
	for i := range numNodes {
		id := p2p.PeerId(getNodeName(i))
		node, err := node.NewActiveNode(id, committee, validators[i], config)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", id, err)
		}
		nodes[i] = node
	}

	// Step 3: start a source for transactions
	log.Info("Starting transaction source", "txPerSecond", txPerSecond)
	rpc := nodes[0].GetRpcService()
	counter := 0
	interval := time.Second / time.Duration(txPerSecond)
	job := concurrent.StartPeriodicJob(interval, func(time.Time) {
		err := rpc.Send(getTx(counter))
		if err != nil {
			log.Warn("Failed to send transaction", "tx_counter", counter, "error", err)
		}
		counter++
	})

	// Step 4: let the simulation run for the requested duration
	time.Sleep(duration)

	// Step 5: shut down the simulation
	log.Info("Stopping transaction source")
	job.Stop()

	log.Info("Stopping nodes")
	for _, node := range nodes {
		node.Stop()
	}

	return nil
}
