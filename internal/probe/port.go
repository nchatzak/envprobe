package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
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
		return Result{Name: p.name, Found: false, Problem: dialProblem(err)}
	}
	_ = conn.Close()
	return Result{Name: p.name, Found: true}
}

// dialProblem names the cause a dial failed. The default names none.
func dialProblem(err error) string {
	switch {
	case isTimeout(err):
		return problemTimedOut
	case errors.Is(err, context.Canceled):
		return problemCancelled
	case errors.Is(err, syscall.ECONNREFUSED):
		return problemConnectionRefused
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return problemUnreachable
	case isType[*net.DNSError](err):
		return problemHostNotFound
	case isType[*net.AddrError](err):
		return problemInvalidAddress
	default:
		return problemConnectionFailed
	}
}

// isTimeout covers every shape a dial timeout arrives in: the context
// deadline, the socket write deadline, and ETIMEDOUT from the OS.
func isTimeout(err error) bool {
	netErr, ok := errors.AsType[net.Error](err)
	return ok && netErr.Timeout()
}

type portConfig struct {
	Target string
}

func newPortCheck(name string, with map[string]any) (Check, error) {
	var cfg portConfig
	decodeErr := decodeWith(with, &cfg)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode port config: %w", decodeErr)
	}

	if cfg.Target == "" {
		return nil, ErrTargetRequired
	}

	return portCheck{
		name:   name,
		target: cfg.Target,
	}, nil
}
