package probe

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/Masterminds/semver/v3"
)

// binaryCheck checks for the presence of a binary in the system PATH and optionally retrieves its version.
type binaryCheck struct {
	name              string
	target            string
	versionArgs       []string // e.g ["--version"] or ["-version"] or ["-v"]
	versionConstraint *semver.Constraints
}

var _ Check = binaryCheck{}

func (c binaryCheck) Run(ctx context.Context) Result {
	path, err := exec.LookPath(c.target)
	result := Result{Name: c.name, Found: err == nil, Path: path}

	if err != nil || len(c.versionArgs) == 0 {
		return result
	}

	output, err := exec.CommandContext(ctx, c.target, c.versionArgs...).CombinedOutput()
	if err != nil {
		result.Problem = problemVersionCommandFailed
		if c.versionConstraint != nil {
			result.Found = false
		}
		return result
	}

	result.Version = parseVersionOutput(string(output))
	if c.versionConstraint == nil {
		return result
	}

	version, err := semver.NewVersion(result.Version)
	if err != nil {
		result.Found = false
		result.Problem = problemCannotCompareVersion
		return result
	}

	if !c.versionConstraint.Check(version) {
		result.Found = false
		result.Problem = problemVersionDoesNotMatch
	}

	return result
}

type binaryConfig struct {
	Target            string
	VersionArgs       []string `mapstructure:"version_args"`
	VersionConstraint string   `mapstructure:"version_constraint"`
}

func newBinaryCheck(name string, with map[string]any) (Check, error) {
	var cfg binaryConfig
	decodeErr := decodeWith(with, &cfg)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode binary config: %w", decodeErr)
	}

	if cfg.Target == "" {
		cfg.Target = name
	}

	var cons *semver.Constraints
	if cfg.VersionConstraint != "" {
		if len(cfg.VersionArgs) == 0 {
			return nil, fmt.Errorf("%w when version_constraint is set", ErrVersionArgsRequired)
		}

		var err error
		cons, err = semver.NewConstraint(cfg.VersionConstraint)
		if err != nil {
			return nil, fmt.Errorf("%w %q", ErrInvalidVersionConstraint, cfg.VersionConstraint)
		}
	}

	return binaryCheck{
		name:              name,
		target:            cfg.Target,
		versionArgs:       cfg.VersionArgs,
		versionConstraint: cons,
	}, nil
}
