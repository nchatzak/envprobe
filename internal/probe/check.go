package probe

import (
	"context"
	"sync"
	"time"
)

type Check interface {
	Run(ctx context.Context) Result
}

type Result struct {
	Name     string
	Found    bool
	Path     string
	Version  string
	Duration time.Duration
	Problem  string
}

// Every value Problem can take. A check sets one of these only when its
// outcome has more than one cause; see docs/decisions.md.
const (
	problemConnectionRefused    = "connection refused"
	problemUnreachable          = "unreachable"
	problemConnectionFailed     = "connection failed"
	problemTimedOut             = "timed out"
	problemCancelled            = "cancelled"
	problemHostNotFound         = "host not found"
	problemInvalidAddress       = "invalid address"
	problemVersionCommandFailed = "version command failed"
	problemDockerNotOnPath      = "docker not on PATH"
	problemDaemonNotResponding  = "daemon not responding"
)

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
