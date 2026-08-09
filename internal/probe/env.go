package probe

import (
	"context"
	"fmt"
	"os"
	"regexp"
)

type envCheck struct {
	name    string
	target  string
	matches *regexp.Regexp // nil when no pattern configured
}

var _ Check = envCheck{}

func (c envCheck) Run(_ context.Context) Result {
	value, ok := os.LookupEnv(c.target)

	if !ok {
		return Result{Name: c.name, Found: false, Problem: problemNotSet}
	}

	if c.matches == nil || c.matches.MatchString(value) {
		return Result{Name: c.name, Found: true}
	}

	if value == "" {
		return Result{Name: c.name, Found: false, Problem: problemSetButEmpty}
	}

	return Result{Name: c.name, Found: false, Problem: problemDoesNotMatch}
}

type envConfig struct {
	Target  string
	Matches string
}

func newEnvCheck(name string, with map[string]any) (Check, error) {
	var cfg envConfig
	decodeErr := decodeWith(with, &cfg)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode env config: %w", decodeErr)
	}

	if cfg.Target == "" {
		cfg.Target = name
	}

	var pattern *regexp.Regexp
	if cfg.Matches != "" {
		var err error
		pattern, err = regexp.Compile(cfg.Matches)
		if err != nil {
			return nil, fmt.Errorf("compile matches: %w", err)
		}
	}

	return envCheck{
		name:    name,
		target:  cfg.Target,
		matches: pattern,
	}, nil
}
