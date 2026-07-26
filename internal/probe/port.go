package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// portCheck checks if a TCP port is open on a given host.
type portCheck struct {
	name   string
	target string // e.g "localhost:8080"
}

var _ Check = portCheck{}

func (p portCheck) Run(ctx context.Context) Result {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", p.target)

	if err != nil {
		return Result{Name: p.name, Found: false}
	}
	conn.Close()
	return Result{Name: p.name, Found: true}
}

type portConfig struct {
	Target string
}

func newPortCheck(name string, with map[string]any) (Check, error) {
	var cfg portConfig
	decodeErr := decodeWith(with, &cfg)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode port config for %q: %w", name, decodeErr)
	}

	if cfg.Target == "" {
		return nil, errors.New("target is required")
	}

	return portCheck{
		name:   name,
		target: cfg.Target,
	}, nil
}
