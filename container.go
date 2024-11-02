package dft

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const intervalWait = 150

type Container struct {
	id           string
	portMappings map[uint][]string
}

func newContainer(
	ctx context.Context,
	imageName string,
	opts ...ContainerOption,
) (*Container, error) {
	cfg := containerCfg{
		args:   nil,
		env:    nil,
		mounts: nil,
		ports:  nil,
	}

	// INFO: we could pass the options further down and parse them in functions
	// we are calling, but we need the exposed ports here to check if we are up
	for i := range opts {
		opts[i](&cfg)
	}

	var (
		arguments    []string
		envVars      []string
		exposedPorts [][2]uint
		mounts       [][2]string
	)

	if cfg.args != nil {
		arguments = *cfg.args
	}

	if cfg.env != nil {
		envVars = *cfg.env
	}

	if cfg.mounts != nil {
		mounts = *cfg.mounts
	}

	if cfg.ports != nil {
		exposedPorts = *cfg.ports
	}

	id, err := startContainer(
		ctx,
		imageName,
		arguments,
		envVars,
		exposedPorts,
		mounts,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"[%s](%s) %w",
			imageName,
			id,
			err,
		)
	}

	// at this point we have a container up
	// but it may not be able to meet our conditions
	// in the given context.
	// In this case we will throw an error and not returning
	// any container reference, therefore we need to take care
	// of its removal
	defer func() {
		if err != nil {
			ctr := Container{id: id}

			sCtx, sCtxCancel := context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
			_ = ctr.Stop(sCtx)
			sCtxCancel()
		}
	}()

	err = containerIsAlive(ctx, id)
	if err != nil {
		l, _ := getLogs(ctx, id)

		return nil, fmt.Errorf(
			"[%s](%s) %w\nlogs:%s",
			imageName,
			id,
			err,
			l,
		)
	}

	prtMpns := map[uint][]string{}

	if len(exposedPorts) > 0 {
		errCh := make(chan error)

		go func() {
			t := time.NewTicker(intervalWait * time.Millisecond)
			defer t.Stop()

			for {
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()

					return
				case <-t.C:
					pm, pErr := getPublishedPorts(ctx, id)
					if pErr != nil {
						errCh <- pErr

						return
					}

					if len(pm) == 0 {
						continue
					}

					prtMpns = pm
					errCh <- nil

					return
				}
			}
		}()

		if err = <-errCh; err != nil {
			l, _ := getLogs(ctx, id)

			return nil, fmt.Errorf(
				"[%s](%s) %w\nlogs:%s",
				imageName,
				id,
				err,
				l,
			)
		}
	}

	// prtMpns, err := getPublishedPorts(ctx, id)
	// if err != nil {
	// 	_ = stopContainer(ctx, id)

	// 	return nil, fmt.Errorf(
	// 		"[%s](%s) %w",
	// 		imageName,
	// 		id,
	// 		err,
	// 	)
	// }

	return &Container{
		id:           id,
		portMappings: prtMpns,
	}, nil
}

// Stop will stop the container and remove it (as well as related volumes)
// from the host system
func (c Container) Stop(ctx context.Context) error {
	err := stopContainer(ctx, c.id)
	if err != nil {
		return err
	}

	ids, err := getVolumes(ctx, c.id)
	if err != nil {
		return err
	}

	err = removeContainer(ctx, c.id)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	return deleteVolumes(ctx, ids)
}

// Logs will retrieve the latest logs from the container
// This call errors once `Stop` was called.
func (c *Container) Logs(ctx context.Context) (string, error) {
	return getLogs(ctx, c.id)
}

// ExposedPorts will return a list of host ports exposing the internal port
func (c *Container) ExposedPorts(port uint) ([]uint, bool) {
	strs, ok := c.portMappings[port]
	if !ok {
		return nil, false
	}

	p := make([]uint, 0, len(strs))

	for i := range strs {
		parts := strings.Split(strs[i], ":")

		v, err := strconv.ParseUint(parts[1], base10, bit64)
		if err != nil {
			return nil, false
		}

		p = append(p, uint(v))
	}

	return p, true
}

// ExposedPortAddresses will return a list of host ports exposing the internal port in the format of
// "<IP>:<PORT>"
func (c *Container) ExposedPortAddresses(port uint) ([]string, bool) {
	str, ok := c.portMappings[port]

	return str, ok
}

// WaitCmd takes a command in the form of a ["<cmd>", "(<arg> | <-flag> | <flagvalue>)"...]
// which will be executed periodically until either
// it returns true
// or the context expires
func (c *Container) WaitCmd(
	ctx context.Context,
	cmd []string,
	metCondition func(stdOut string, stdErr string, code int) bool,
	opts ...WaitOption,
) error {
	cfg := waitCfg{
		inContainer: nil,
	}

	for i := range opts {
		opts[i](&cfg)
	}

	errCh := make(chan error)

	go func(inContainer bool) {
		t := time.NewTicker(intervalWait * time.Millisecond)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()

				return
			case <-t.C:
				var (
					outB bytes.Buffer
					errB bytes.Buffer
					code int
					err  error
				)

				if inContainer {
					// call docker exec
					outB, errB, code, err = dockerExecute(ctx, c.id, cmd)
				} else {
					// call func on host
					outB, errB, code, err = hostExecute(ctx, cmd)
				}

				if err != nil && code == -1 {
					errCh <- fmt.Errorf(
						"wait command errored: %w\n\tstdErr:%s\n\tstdOut:%s",
						err,
						errB.String(),
						outB.String(),
					)

					return
				}

				if !metCondition(outB.String(), errB.String(), code) {
					continue
				}

				errCh <- nil

				return
			}
		}
	}(cfg.inContainer != nil && *cfg.inContainer)

	return <-errCh
}
