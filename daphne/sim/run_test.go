package sim

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"
)

func TestRun_SmokeTest(t *testing.T) {
	output := filepath.Join(t.TempDir(), "output.csv")
	command := getRunCommand()
	require.NotNil(t, command)
	require.NoError(t, command.Run(t.Context(), []string{
		"run", "-s",
		"-o", output,
	}))
	require.FileExists(t, output)
}

func TestRun_InvalidOutputLocation_ReportsOutputError(t *testing.T) {
	command := getRunCommand()
	require.NotNil(t, command)
	err := command.Run(t.Context(), []string{
		"run", "-d", "100ms",
		"-o", t.TempDir(), // < can not write to a directory
	})
	require.ErrorContains(t, err, "is a directory")
}

func TestRunScenario_ForwardsErrorIfScenarioFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	scenario := scenario.NewMockScenario(ctrl)

	issue := fmt.Errorf("scenario failed")
	scenario.EXPECT().Run(gomock.Any(), gomock.Any()).Return(issue)
	require.ErrorIs(t, runScenario(&cli.Command{}, scenario), issue)
}

func TestExportData_RegularDataWithValidDirectory_SuccessfulDataExport(t *testing.T) {
	require := require.New(t)
	data := []tracker.Entry{
		{Time: time.Unix(0, 1), Mark: mark.MsgSent},
		{Time: time.Unix(0, 2), Mark: mark.MsgReceived},
	}
	outputFile := filepath.Join(t.TempDir(), "output.csv")
	require.NoError(exportData(data, outputFile))

	out := &bytes.Buffer{}
	require.NoError(tracker.ExportAsCSV(data, out))
	got, err := os.ReadFile(outputFile)
	require.NoError(err)
	require.Equal(out.Bytes(), got)
}

func TestExportData_InvalidName_reportsAnError(t *testing.T) {
	require := require.New(t)
	err := exportData(nil, t.TempDir())
	require.ErrorContains(err, "is a directory")
}
