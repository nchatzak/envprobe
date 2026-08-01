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
