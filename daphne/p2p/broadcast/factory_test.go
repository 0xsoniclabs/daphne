package broadcast

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestNewDefault_UsesDefaultFactoryForAllKeyMessageCombinations(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	intToInt := func(int) int { return 0 }
	require.IsType(
		DefaultFactory[int, int]()(p2pServer, intToInt),
		NewDefault(p2pServer, intToInt),
	)

	intToString := func(int) string { return "" }
	require.IsType(
		DefaultFactory[string, int]()(p2pServer, intToString),
		NewDefault(p2pServer, intToString),
	)

	boolToString := func(bool) string { return "" }
	require.IsType(
		DefaultFactory[string, bool]()(p2pServer, boolToString),
		NewDefault(p2pServer, boolToString),
	)
}
