package doctor

import (
	"context"
	"sync"
	"time"
)

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

// CountFailed reports how many checks did not pass.
func CountFailed(results []Result) int {
	count := 0
	for _, result := range results {
		if !result.Found {
			count++
		}
	}
	return count
}

const defaultTimeout = 5 * time.Second

func RunAll(ctx context.Context, checks []Check) []Result {
	results := make([]Result, len(checks))
	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
			start := time.Now()
			result := check.Run(ctx)
			result.Duration = time.Since(start)
			results[i] = result
		})
	}
	wg.Wait()
	return results
}

func DefaultChecks() []Check {
	return []Check{
		binaryCheck{name: "Git", target: "git", versionArgs: []string{"--version"}},
		binaryCheck{name: "Java", target: "java", versionArgs: []string{"-version"}},
		binaryCheck{name: "Go", target: "go", versionArgs: []string{"version"}},
		dockerDaemonCheck{},
	}
}

func SelectChecks(all []Check, names []string) []Check {
	if len(names) == 0 {
		return all
	}
	selected := make([]Check, 0, len(names))
	nameSet := make(map[string]bool, len(names))
	for _, name := range names {
		nameSet[name] = true
	}
	for _, check := range all {
		if nameSet[check.Name()] {
			selected = append(selected, check)
		}
	}
	return selected
}
