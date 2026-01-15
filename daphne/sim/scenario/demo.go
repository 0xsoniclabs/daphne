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
	StateProcessingDelayModel state.ProcessingDelayModel
	NetworkGeography          NetworkGeography

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
		getTx = defaultTransactionGenerator
	}

	log.Info(
		"Starting Demo Scenario",
		"numValidators", numValidators,
		"numRpcNodes", numRpcNodes,
		"numObservers", numObservers,
		"txPerSecond", txPerSecond,
		"duration", duration,
		"consensus", consensusFactory,
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

	// Add validator nodes participating in consensus.
	validators := committee.Validators()
	validatorPeerIds := []p2p.PeerId{}
	for i := range numValidators {
		validatorPeerIds = append(validatorPeerIds, p2p.PeerId(getNodeName("V", i)))
	}

	if d.NetworkGeography.GetNumRegions() != 0 {
		builder = builder.WithLatency(getNetworkLatencyModel(validatorPeerIds, d.NetworkGeography))
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

	nodes := []*node.Node{}
	for i := range numValidators {
		node, err := node.NewActiveNode(validatorPeerIds[i], committee, validators[i], config)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", validatorPeerIds[i], err)
		}
		nodes = append(nodes, node)
	}

	// Add RPC nodes providing entry points for transactions.
	rpcs := []rpc.Server{}
	nonValidatorIds := []p2p.PeerId{}
	for i := range numRpcNodes {
		id := p2p.PeerId(getNodeName("R", i))
		node, err := node.NewPassiveNode(id, committee, config)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", id, err)
		}
		nodes = append(nodes, node)
		rpcs = append(rpcs, node.GetRpcService())
		nonValidatorIds = append(nonValidatorIds, id)
	}

	// Add observers participating in the network, but not contributing.
	for i := range numObservers {
		id := p2p.PeerId(getNodeName("O", i))
		node, err := node.NewPassiveNode(id, committee, config)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", id, err)
		}
		nodes = append(nodes, node)
		nonValidatorIds = append(nonValidatorIds, id)
	}

	// Configure network topology now that all nodes have been created.
	// Use provided topology factory, or default to fully-meshed
	if d.Topology != nil {
		// Convert peer slice to map with all peers in layer 0
		peerMap := make(map[p2p.PeerId]int, len(nonValidatorIds)+len(validatorPeerIds))
		for _, peerId := range validatorPeerIds {
			peerMap[peerId] = p2p.TwoLayerValidatorLayer
		}
		for _, peerId := range nonValidatorIds {
			peerMap[peerId] = p2p.TwoLayerNonValidatorLayer
		}
		topology := d.Topology.Create(peerMap)
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

// defaultTransactionGenerator generates transactions for the demo scenario by
// mixing a single power-user sending 50% of all transactions, and 99 other
// users sending the remaining 50% in a round-robin fashion.
func defaultTransactionGenerator(i int) types.Transaction {
	const NumSenders = 100
	if i%2 == 0 {
		return singleUserTransactionGenerator(i / 2)
	}
	tx := roundRobbingTransactionGenerator(NumSenders-1, i/2)
	tx.From += 1 // < offset by one to avoid colliding with power user
	return tx
}

// singleUserTransactionGenerator generates transactions from a single user
// with sender address 0.
func singleUserTransactionGenerator(i int) types.Transaction {
	return types.Transaction{
		From:  types.Address(0),
		To:    1,
		Nonce: types.Nonce(i),
	}
}

// roundRobbingTransactionGenerator generates transactions from multiple users
// in a round-robbing fashion with sender address 0 to numSenders-1.
func roundRobbingTransactionGenerator(
	numSenders int,
	i int,
) types.Transaction {
	return types.Transaction{
		From:  types.Address(i % numSenders),
		To:    1,
		Nonce: types.Nonce(i / numSenders),
	}
}

func getNetworkLatencyModel(peers []p2p.PeerId, geography NetworkGeography) *p2p.DelayModel {
	defaultLatency := p2p.NewDelayModel().
		SetBaseDeliveryDistribution(geography.GetLocalDeliveryLatency()).
		SetBaseSendDistribution(geography.GetLocalSendLatency())

	// Partition validators into regions
	regions := make([][]p2p.PeerId, geography.GetNumRegions())
	for i, validatorPeerId := range peers {
		regionId := i % geography.GetNumRegions()
		regions[regionId] = append(regions[regionId], validatorPeerId)
	}

	for regionId := range len(regions) {
		for otherRegionId := regionId + 1; otherRegionId < len(regions); otherRegionId++ {
			regionPeers := regions[regionId]
			otherRegionPeers := regions[otherRegionId]
			for _, peer := range regionPeers {
				for _, otherPeer := range otherRegionPeers {
					defaultLatency.SetConnectionSendDistribution(peer, otherPeer, geography.GetInterRegionSendLatency())
					defaultLatency.SetConnectionSendDistribution(otherPeer, peer, geography.GetInterRegionSendLatency())

					defaultLatency.SetConnectionDeliveryDistribution(peer, otherPeer, geography.GetInterRegionDeliveryLatency())
					defaultLatency.SetConnectionDeliveryDistribution(otherPeer, peer, geography.GetInterRegionDeliveryLatency())
				}
			}
		}
	}

	return defaultLatency
}
