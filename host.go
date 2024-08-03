package dft

import (
	"bytes"
	"context"
	"os/exec"
)

func hostExecute(
	ctx context.Context,
	command []string,
) (
	stdOutCapture bytes.Buffer,
	stdErrCapture bytes.Buffer,
	exitCode int,
	err error,
) {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...) // nolint:gosec

	cmd.Stderr = &stdErrCapture
	cmd.Stdout = &stdOutCapture

	err = cmd.Run()

	return stdOutCapture, stdErrCapture, cmd.ProcessState.ExitCode(), err
}
