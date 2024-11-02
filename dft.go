package dft

import (
	"context"
	"os/exec"
)

const dockerCmd = "docker"

// StartContainer tries to spin up a container for the given image.
// This may take a while if the given image is not present on the host
// since it will be pulled from the registry.
func StartContainer(
	ctx context.Context,
	imageName string,
	opts ...ContainerOption,
) (*Container, error) {
	if _, err := exec.LookPath(dockerCmd); err != nil {
		return nil, err
	}

	return newContainer(
		ctx,
		imageName,
		opts...,
	)
}
