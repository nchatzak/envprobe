package doctor

import (
	"context"
	"os/exec"
	"time"
)

type binaryCheck struct {
	name        string
	versionArgs []string // e.g ["--version"] or ["-version"] or ["-v"]
}

var _ Check = binaryCheck{}

func (c binaryCheck) Name() string {
	return c.name
}

func (c binaryCheck) Run(ctx context.Context) Result {
	result := checkTool(c.name)
	if result.Found && len(c.versionArgs) > 0 {
		output, err := exec.CommandContext(ctx, c.name, c.versionArgs...).CombinedOutput()
		if err == nil {
			result.Version = parseVersionOutput(string(output))
		}
	}
	return result
}

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

type Check interface {
	Name() string
	Run(ctx context.Context) Result
}

type Result struct {
	Name     string
	Found    bool
	Path     string
	Version  string
	Duration time.Duration
}

func RunAll(ctx context.Context, checks []Check) []Result {
	results := make([]Result, 0, len(checks))
	for _, check := range checks {
		start := time.Now()
		result := check.Run(ctx)
		result.Duration = time.Since(start)
		results = append(results, result)
	}

	return results
}

func DefaultChecks() []Check {
	return []Check{
		binaryCheck{name: "git", versionArgs: []string{"--version"}},
		binaryCheck{name: "java", versionArgs: []string{"-version"}},
		binaryCheck{name: "go", versionArgs: []string{"version"}},
		dockerDaemonCheck{},
	}
}

func checkTool(tool string) Result {
	path, err := exec.LookPath(tool)
	return Result{
		Name:  tool,
		Found: err == nil,
		Path:  path,
	}
}
