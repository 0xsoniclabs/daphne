package broadcast

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestFactories_UsesDefaultFactoryForAllKeyMessageCombinations(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	var factories Factories // < uses default for everything

	intToInt := func(int) int { return 0 }
	require.IsType(
		NewDefault(p2pServer, intToInt),
		NewChannel(factories, p2pServer, intToInt),
	)

	intToString := func(int) string { return "" }
	require.IsType(
		NewDefault(p2pServer, intToString),
		NewChannel(factories, p2pServer, intToString),
	)

	boolToString := func(bool) string { return "" }
	require.IsType(
		NewDefault(p2pServer, boolToString),
		NewChannel(factories, p2pServer, boolToString),
	)
}

func TestFactories_SetFactory_OverridesDefaultFactory(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	var factories Factories // < uses default for everything
	factories = SetFactory(factories, NewForwarding[int, int])

	// Test that int->int uses Forwarding
	intToInt := func(int) int { return 0 }
	require.IsType(
		NewForwarding(p2pServer, intToInt),
		NewChannel(factories, p2pServer, intToInt),
	)

	// Test that int->string still uses Default
	intToString := func(int) string { return "" }
	require.IsType(
		NewDefault(p2pServer, intToString),
		NewChannel(factories, p2pServer, intToString),
	)

	// Factory setups can be updated multiple times.
	factories = SetFactory(factories, NewGossip[int, int])

	// Test that int->int now uses Gossip
	require.IsType(
		NewGossip(p2pServer, intToInt),
		NewChannel(factories, p2pServer, intToInt),
	)
}
