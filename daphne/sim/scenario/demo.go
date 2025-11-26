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
	"github.com/0xsoniclabs/daphne/daphne/rpc"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// DemoScenario is a simple scenario starting a fixed number of nodes in a
// network with a configurable topology to which transactions are sent at a
// fixed rate.
type DemoScenario struct {
	NumValidators             int
	NumRpcNodes               int
	NumObservers              int
	TxPerSecond               int
	Duration                  time.Duration
	Broadcast                 broadcast.Protocol
	Consensus                 consensus.Factory
	Topology                  p2p.TopologyFactory
	NetworkLatencyModel       p2p.LatencyModel
	StateProcessingDelayModel state.ProcessingDelayModel

	nodeNameGenerator    func(prefix string, index int) string
	transactionGenerator func(int) types.Transaction
}

func (d *DemoScenario) Run(
	log Logger,
	tracker tracker.Tracker,
) error {
	// Handle default parameters.
	numValidators := d.NumValidators
	if numValidators <= 0 {
		numValidators = 3
	}
	numRpcNodes := max(1, d.NumRpcNodes)
	numObservers := max(0, d.NumObservers)

	txPerSecond := d.TxPerSecond
	if txPerSecond <= 0 {
		txPerSecond = 100
	}
	duration := d.Duration
	if duration <= 0 {
		duration = 5 * time.Second
	}

	committee := *consensus.NewUniformCommittee(numValidators)

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
		getNodeName = func(prefix string, i int) string {
			return fmt.Sprintf("%s-%03d", prefix, i+1)
		}
	}

	getTx := d.transactionGenerator
	if getTx == nil {
		getTx = func(i int) types.Transaction {
			return types.Transaction{From: 0, To: 1, Nonce: types.Nonce(i)}
		}
	}

	log.Info(
		"Starting Demo Scenario",
		"numValidators", numValidators,
		"numRpcNodes", numRpcNodes,
		"numObservers", numObservers,
		"txPerSecond", txPerSecond,
		"duration", duration,
	)

	// Step 1: define genesis data for the demo
	genesis := state.Genesis{}

	// Step 2: set up a network of nodes.
	log.Info(
		"Setting up network",
		"numValidators", numValidators,
		"numRpcNodes", numRpcNodes,
		"numObservers", numObservers,
	)
	builder := p2p.NewNetworkBuilder().WithTracker(tracker)

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

	// Add validator nodes participating in consensus.
	validators := committee.Validators()
	nodes := []*node.Node{}
	peerIds := []p2p.PeerId{}
	for i := range numValidators {
		id := p2p.PeerId(getNodeName("V", i))
		node, err := node.NewActiveNode(id, committee, validators[i], config)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", id, err)
		}
		nodes = append(nodes, node)
		peerIds = append(peerIds, id)
	}

	// Add RPC nodes providing entry points for transactions.
	rpcs := []rpc.Server{}
	for i := range numRpcNodes {
		id := p2p.PeerId(getNodeName("R", i))
		node, err := node.NewPassiveNode(id, committee, config)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", id, err)
		}
		nodes = append(nodes, node)
		rpcs = append(rpcs, node.GetRpcService())
		peerIds = append(peerIds, id)
	}

	// Add observers participating in the network, but not contributing.
	for i := range numObservers {
		id := p2p.PeerId(getNodeName("O", i))
		node, err := node.NewPassiveNode(id, committee, config)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", id, err)
		}
		nodes = append(nodes, node)
		peerIds = append(peerIds, id)
	}

	// Configure network topology now that all nodes have been created.
	// Use provided topology factory, or default to fully-meshed
	if d.Topology != nil {
		topology := d.Topology.Create(peerIds)
		network.UpdateTopology(topology)
	}

	// Step 3: start a source for transactions
	log.Info("Starting transaction source", "txPerSecond", txPerSecond)
	counter := 0
	interval := time.Second / time.Duration(txPerSecond)
	job := concurrent.StartPeriodicJob(interval, func(time.Time) {
		rpc := rpcs[counter%len(rpcs)]
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
