package probe

import (
	"context"
	"fmt"
	"os/exec"
)

// binaryCheck checks for the presence of a binary in the system PATH and optionally retrieves its version.
type binaryCheck struct {
	name        string
	target      string
	versionArgs []string // e.g ["--version"] or ["-version"] or ["-v"]
}

var _ Check = binaryCheck{}

func (c binaryCheck) Run(ctx context.Context) Result {
	result := checkTool(c.target)
	result.Name = c.name

	if result.Found && len(c.versionArgs) > 0 {
		output, err := exec.CommandContext(ctx, c.target, c.versionArgs...).CombinedOutput()
		if err == nil {
			result.Version = parseVersionOutput(string(output))
		}
	}

	return result
}

func checkTool(tool string) Result {
	path, err := exec.LookPath(tool)
	return Result{
		Name:  tool,
		Found: err == nil,
		Path:  path,
	}
}

type binaryConfig struct {
	Target      string
	VersionArgs []string `mapstructure:"version_args"`
}

func newBinaryCheck(name string, with map[string]any) (Check, error) {
	var cfg binaryConfig
	decodeErr := decodeWith(with, &cfg)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode binary config for %q: %w", name, decodeErr)
	}

	if cfg.Target == "" {
		cfg.Target = name
	}

	return binaryCheck{
		name:        name,
		target:      cfg.Target,
		versionArgs: cfg.VersionArgs,
	}, nil
}
