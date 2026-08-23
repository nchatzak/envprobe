package probe

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func TestDockerProblem(t *testing.T) {
	notOnPath := &exec.Error{Name: "docker", Err: exec.ErrNotFound}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"daemon answered", nil, ""},
		{"docker not on PATH", notOnPath, problemDockerNotOnPath},
		{"lookup failure wrapped", fmt.Errorf("run docker info: %w", notOnPath), problemDockerNotOnPath},
		// docker info found the binary and exited non-zero: the daemon is the
		// one not answering.
		{"non-zero exit", &exec.ExitError{}, problemDaemonNotResponding},
		{"anything else", errors.New("boom"), problemDaemonNotResponding},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dockerProblem(tt.err); got != tt.want {
				t.Errorf("dockerProblem(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
