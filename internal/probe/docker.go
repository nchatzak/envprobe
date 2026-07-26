package probe

import (
	"context"
	"fmt"
	"os/exec"
)

// dockerDaemonCheck checks if the Docker daemon is running by executing "docker info".
type dockerDaemonCheck struct {
	name string
}

var _ Check = dockerDaemonCheck{}

func (c dockerDaemonCheck) Run(ctx context.Context) Result {
	cmd := exec.CommandContext(ctx, "docker", "info")
	err := cmd.Run()

	return Result{
		Name:  c.name,
		Found: err == nil,
	}
}

// dockerDaemonConfig takes no fields: the check is fully described by its name.
// Decoding into it anyway makes ErrorUnused reject a misspelled key, so this
// kind fails as loudly as the others.
type dockerDaemonConfig struct{}

func newDockerDaemonCheck(name string, with map[string]any) (Check, error) {
	var cfg dockerDaemonConfig
	if decodeErr := decodeWith(with, &cfg); decodeErr != nil {
		return nil, fmt.Errorf("decode docker-daemon config for %q: %w", name, decodeErr)
	}

	return dockerDaemonCheck{name: name}, nil
}
