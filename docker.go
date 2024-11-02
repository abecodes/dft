package dft

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	actionContainer = "container"
	actionExec      = "exec"
	actionInspect   = "inspect"
	actionPort      = "port"
	actionRun       = "run"
	actionVolume    = "volume"

	idLength      = 12
	intervalAlive = 200

	stateCreated    = "'created'\n"
	stateDead       = "'dead'\n"
	stateExited     = "'exited'\n"
	statePaused     = "'paused'\n"
	stateRestarting = "'restarting'\n"
	stateRunning    = "'running'\n"
)

func startContainer(
	ctx context.Context,
	imageName string,
	arguments []string,
	envVars []string,
	exposedPorts [][2]uint,
	mounts [][2]string,
) (string, error) {
	var (
		stdOutCapture bytes.Buffer
		stdErrCapture bytes.Buffer
	)

	// with `--rm` the container and its *anonymous* volumes
	// get removed by docker, no need for extensive cleanup
	// args := []string{runAction, "-d", "--rm"}
	// INFO: but if we use `--rm`, we loose the ability to dump logs
	args := []string{actionRun, "-d"}

	for i := range exposedPorts {
		var seq string

		if exposedPorts[i][1] == 0 {
			// use random host port to expose container port
			seq = strconv.FormatUint(uint64(exposedPorts[i][0]), base10)
		} else {
			// use specific host port to expose container port
			seq = strconv.FormatUint(uint64(exposedPorts[i][0]), base10) +
				":" +
				strconv.FormatUint(uint64(exposedPorts[i][1]), base10)
		}

		args = append(args, "-p", seq)
	}

	// passing envVars
	for i := range envVars {
		args = append(args, "-e", envVars[i])
	}

	// passing envVars
	for i := range mounts {
		args = append(
			args,
			"--mount",
			fmt.Sprintf(
				"type=bind,source=%s,target=%s",
				mounts[i][0],
				mounts[i][1],
			),
		)
	}

	args = append(args, imageName)

	// appending command overwrites
	// (overwriting dockerfile [CMD])
	args = append(args, arguments...)

	cmd := exec.CommandContext(
		ctx,
		dockerCmd,
		args...,
	)

	cmd.Stderr = &stdErrCapture
	cmd.Stdout = &stdOutCapture

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf(
			"unable to start container:\n%s\n%s\nargs: %q",
			stdOutCapture.String(),
			stdErrCapture.String(),
			strings.Join(args, " "),
		)
	}

	return stdOutCapture.String()[:idLength], nil
}

func containerIsAlive(
	ctx context.Context,
	id string,
) error {
	t := time.NewTicker(intervalAlive * time.Millisecond)
	rdy := make(chan struct{})
	err := make(chan error)

	go func() {
		for range t.C {
			var stdOutCapture bytes.Buffer
			var stdErrCapture bytes.Buffer

			cmd := exec.CommandContext(
				ctx,
				dockerCmd,
				actionInspect,
				"-f",
				"'{{.State.Status}}'",
				id,
			)

			cmd.Stdout = &stdOutCapture
			cmd.Stderr = &stdErrCapture

			cErr := cmd.Run()
			if cErr != nil {
				err <- errors.Join(
					cErr,
					fmt.Errorf(
						"unable to inspect container: %s",
						stdErrCapture.String(),
					),
				)

				t.Stop()

				return
			}

			state := stdOutCapture.String()

			switch state {
			case stateDead,
				stateExited,
				statePaused,
				stateRestarting:
				err <- fmt.Errorf(
					"container in invalid state: %s\nstdOut:%s\nstdErr:%s",
					state,
					stdOutCapture.String(),
					stdErrCapture.String(),
				)

				t.Stop()

				return
			case stateRunning:
				rdy <- struct{}{}

				t.Stop()

				return
			}
		}
	}()

	select {
	case e := <-err:
		return e
	case <-rdy:
	}

	return nil
}

func getPublishedPorts(
	ctx context.Context,
	id string,
) (map[uint][]string, error) {
	var (
		stdOutCapture bytes.Buffer
		stdErrCapture bytes.Buffer
	)

	portMappings := map[uint][]string{}

	cmd := exec.CommandContext(ctx, dockerCmd, actionPort, id)

	cmd.Stderr = &stdErrCapture
	cmd.Stdout = &stdOutCapture

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf(
			"unable to retrieve container ports: %s",
			stdErrCapture.String(),
		)
	}

	stdErrCapture.Reset()

	s := bufio.NewScanner(&stdOutCapture)

	for s.Scan() {
		t := s.Text()
		parts := strings.Split(t, " -> ")
		port, pErr := strconv.ParseUint(
			strings.ReplaceAll(parts[0], "/tcp", ""),
			base10,
			bit64,
		)
		if pErr != nil {
			return nil, err
		}

		portMappings[uint(port)] = append(portMappings[uint(port)], parts[1])
	}

	if err = s.Err(); err != nil {
		return nil, err
	}

	return portMappings, nil
}

func getLogs(ctx context.Context, id string) (string, error) {
	out, err := exec.CommandContext(ctx, dockerCmd, "logs", id).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"unable to retrieve logs for container %s: %w",
			id,
			err,
		)
	}

	return string(out), nil
}

func getVolumes(ctx context.Context, id string) ([]string, error) {
	var (
		stdOutCapture bytes.Buffer
		stdErrCapture bytes.Buffer
	)

	cmd := exec.CommandContext(
		ctx,
		dockerCmd,
		actionInspect,
		"-f",
		`{{ range .Mounts }}{{if eq .Type "volume"}}{{ .Name }}{{"\n"}}{{ end }}{{ end }}`,
		id,
	)

	cmd.Stderr = &stdErrCapture
	cmd.Stdout = &stdOutCapture

	err := cmd.Run()
	if err != nil {
		return []string(nil), fmt.Errorf(
			"unable to inspect container: %s\nargs: %v",
			stdErrCapture.String(),
			cmd.Args,
		)
	}

	stdErrCapture.Reset()

	volumes := []string{}
	s := bufio.NewScanner(&stdOutCapture)

	for s.Scan() {
		t := s.Text()

		if t != "" {
			volumes = append(volumes, t)
		}
	}

	if err = s.Err(); err != nil {
		return nil, err
	}

	return volumes, nil
}

func deleteVolumes(ctx context.Context, ids []string) error {
	var stdErrCapture bytes.Buffer

	args := []string{
		actionVolume,
		"rm",
	}

	args = append(args, ids...)

	cmd := exec.CommandContext( // nolint:gosec
		ctx,
		dockerCmd,
		args...,
	)

	cmd.Stderr = &stdErrCapture

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(
			"unable to inspect container: %s",
			stdErrCapture.String(),
		)
	}

	return nil
}

func stopContainer(ctx context.Context, id string) error {
	var stdErrCapture bytes.Buffer

	cmd := exec.CommandContext(ctx, dockerCmd, actionContainer, "stop", id)

	cmd.Stderr = &stdErrCapture

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(
			"unable to stop container: %s",
			stdErrCapture.String(),
		)
	}

	return nil
}

func removeContainer(ctx context.Context, id string) error {
	var stdErrCapture bytes.Buffer

	cmd := exec.CommandContext(ctx, dockerCmd, actionContainer, "remove", id)

	cmd.Stderr = &stdErrCapture

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(
			"unable to remove container: %s",
			stdErrCapture.String(),
		)
	}

	return nil
}

func dockerExecute(
	ctx context.Context,
	id string,
	command []string,
) (
	stdOutCapture bytes.Buffer,
	stdErrCapture bytes.Buffer,
	exitCode int,
	err error,
) {
	cmd := exec.CommandContext( // nolint:gosec
		ctx,
		dockerCmd,
		append([]string{actionExec, id}, command...)...,
	)

	cmd.Stderr = &stdErrCapture
	cmd.Stdout = &stdOutCapture

	err = cmd.Run()

	return stdOutCapture, stdErrCapture, cmd.ProcessState.ExitCode(), err
}
