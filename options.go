package dft

import "strings"

type containerCfg struct {
	args  *[]string
	env   *[]string
	ports *[][2]uint
}

type waitCfg struct {
	// inContainer indicates if we need to execute the cmd
	// inside of the container (true) or on the host (false)
	//
	// default: false
	inContainer *bool
}

type (
	ContainerOption func(cfg *containerCfg)
	WaitOption      func(cfg *waitCfg)
)

// WithCmd overwrites the [CMD] part of the dockerfile
func WithCmd(args []string) ContainerOption {
	return func(cfg *containerCfg) {
		if cfg.args == nil {
			cfg.args = new([]string)
		}

		n := append(
			*cfg.args,
			args...,
		)

		cfg.args = &n
	}
}

// WithEnvVar will add "<KEY>=<VALUE>" to the env of the container
func WithEnvVar(key string, value string) ContainerOption {
	return func(cfg *containerCfg) {
		if cfg.env == nil {
			cfg.env = new([]string)
		}

		n := append(
			*cfg.env,
			strings.ToUpper(key)+"="+value,
		)

		cfg.env = &n
	}
}

// WithPort will expose the passed internal port via a given target port on the host.
func WithPort(port uint, target uint) ContainerOption {
	return func(cfg *containerCfg) {
		if cfg.ports == nil {
			cfg.ports = new([][2]uint)
		}

		n := append(*cfg.ports, [2]uint{port, target})

		cfg.ports = &n
	}
}

// WithRandomPort will expose the passed internal port via a random port on the host.
// Use
//
//	ExposedPort
//	ExposedPortAddr
//
// to retrieve the actual host port used
//
// (shorthand for `WithPort(x,0)`)
func WithRandomPort(port uint) ContainerOption {
	return WithPort(port, 0)
}

// WithExecuteInsideContainer defines if the wait cmd is executed inside the container
// or on the host machine
func WithExecuteInsideContainer(b bool) WaitOption {
	return func(cfg *waitCfg) {
		cfg.inContainer = &b
	}
}
