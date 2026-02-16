// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain_CalledWithNoParameters_ReturnsStatusCodeZero(t *testing.T) {
	out, code := runMainWithFlag(t, "")
	require.Contains(t, out, "daphne - A simulation tool for blockchains")
	require.Equal(t, 0, code)
}

func TestMain_CalledWithHelpFlag_ReturnsStatusCodeZero(t *testing.T) {
	out, code := runMainWithFlag(t, "-help")
	require.Contains(t, out, "daphne - A simulation tool for blockchains")
	require.Equal(t, 0, code)
}

func TestMain_CalledWithUnsupportedFlag_ReturnsStatusCodeOne(t *testing.T) {
	out, code := runMainWithFlag(t, "-something-that-is-not-supported")
	require.Equal(t, 1, code)
	require.Contains(t, out, "Failed to run Daphne")
	require.Contains(t, out, "-something-that-is-not-supported")
}

// runMainWithFlag starts this test in a sub-process running only the
// TestRunMain function and returns the exist code.
func runMainWithFlag(t *testing.T, flag string) (output string, code int) {
	require := require.New(t)
	path, err := os.Executable()
	require.NoError(err)

	cmd := exec.Command(path, "-test.run", "TestRunMain", "-daphne.main.flag", flag)
	outBuf := new(bytes.Buffer)
	cmd.Stdout = outBuf
	errBuf := new(bytes.Buffer)
	cmd.Stderr = errBuf

	// Run the command and capture the error code.
	err = cmd.Run()
	if err == nil {
		return outBuf.String() + errBuf.String(), 0
	}
	if err := err.(*exec.ExitError); err != nil {
		return outBuf.String() + errBuf.String(), err.ExitCode()
	}
	require.NoError(err) // should not happen
	return "", 0
}

var (
	flagForMain = flag.String("daphne.main.flag", "DEFAULT", "if not empty, a flag to be passed to main")
)

func TestRunMain(t *testing.T) {
	// This flag is only used when running tests as sub-processes with a custom
	// value for the -daphne.main.flag.
	flag.Parse()
	if *flagForMain == "DEFAULT" {
		t.Skip("skipped, as this test is not called as a sub-process of another test")
	}

	backup := os.Args
	defer func() { os.Args = backup }()

	args := []string{"daphne"}
	if len(*flagForMain) > 0 {
		args = append(args, *flagForMain)
	}
	os.Args = args
	main()
}
