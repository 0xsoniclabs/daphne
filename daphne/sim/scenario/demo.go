package scenario

import (
	"fmt"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/node"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// DemoScenario is a simple scenario starting a fixed number of nodes in a fully
// meshed network to which transactions are send at a fixed rate.
type DemoScenario struct {
	NumNodes    int
	TxPerSecond int
	Duration    time.Duration

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

	// Step 1: set up a network of 3 nodes.
	log.Info("Setting up network", "numNodes", numNodes)
	network := p2p.NewNetworkBuilder().WithTracker(tracker).Build()
	nodes := make([]*node.Node, numNodes)
	for i := range numNodes {
		id := p2p.PeerId(getNodeName(i))
		node, err := node.NewPassiveNode(id, network, central.Factory{})
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", id, err)
		}
		nodes[i] = node
	}

	// Step 2: start a source for transactions
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

	time.Sleep(duration)

	log.Info("Stopping transaction source")
	job.Stop()
	return nil
}
