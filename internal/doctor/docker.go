package doctor

import (
	"context"
	"os/exec"
)

// dockerDaemonCheck checks if the Docker daemon is running by executing "docker info".
type dockerDaemonCheck struct{}

var _ Check = dockerDaemonCheck{}

func (c dockerDaemonCheck) Name() string {
	return "docker-daemon"
}

func (c dockerDaemonCheck) Run(ctx context.Context) Result {
	cmd := exec.CommandContext(ctx, "docker", "info")
	err := cmd.Run()

	return Result{
		Name:  c.Name(),
		Found: err == nil,
	}
}

func newDockerDaemonCheck(name string, with map[string]any) (Check, error) {
	return dockerDaemonCheck{}, nil
}
