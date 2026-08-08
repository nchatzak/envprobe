package probe

import (
	"context"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestPortCheck(t *testing.T) {
	t.Run("Port open", func(t *testing.T) {
		listener := openListener(t)
		target := listener.Addr().String()
		check := portCheck{name: "test-port-open", target: target}

		result := check.Run(t.Context())
		if !result.Found {
			t.Errorf("Expected target %s to be accessible", target)
		}
		if result.Problem != "" {
			t.Errorf("Problem = %q, want empty", result.Problem)
		}
	})

	t.Run("Port closed", func(t *testing.T) {
		listener := openListener(t)
		target := listener.Addr().String()
		_ = listener.Close() // Close the listener to simulate a closed port
		check := portCheck{name: "test-port-closed", target: target}

		result := check.Run(t.Context())
		if result.Found {
			t.Errorf("Expected target %s to not be accessible", target)
		}
		if result.Problem != problemConnectionRefused {
			t.Errorf("Unexpected problem %q", result.Problem)
		}
	})

	t.Run("Context Cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel() // Cancel the context
		listener := openListener(t)
		check := portCheck{name: "test-port-cancelled", target: listener.Addr().String()}

		result := check.Run(ctx)
		if result.Found {
			t.Errorf("Expected target %s to not be accessible", check.target)
		}
		if result.Problem != problemCancelled {
			t.Errorf("Unexpected problem %q", result.Problem)
		}
	})

	// The deadline is spent before Run dials, so DialContext returns the
	// context's error without racing the socket deadline. That determinism is
	// the point here; TestDialProblem covers the shapes it cannot produce.
	t.Run("Deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
		defer cancel()
		listener := openListener(t)
		check := portCheck{name: "test-port-timeout", target: listener.Addr().String()}

		result := check.Run(ctx)
		if result.Found {
			t.Errorf("Expected target %s to not be accessible", check.target)
		}
		if result.Problem != problemTimedOut {
			t.Errorf("Unexpected problem %q", result.Problem)
		}
	})

	t.Run("Malformed target", func(t *testing.T) {
		check := portCheck{name: "test-port-malformed", target: "not-a-valid-target"}

		result := check.Run(t.Context())
		if result.Found {
			t.Errorf("Expected target %s to not be accessible", check.target)
		}
		if result.Problem != problemInvalidAddress {
			t.Errorf("Unexpected problem %q", result.Problem)
		}
	})
}

// The subtests above prove the real dialer produces the errors dialProblem
// expects, for the causes a test can provoke locally. The rest cannot be
// provoked without a network, so they are mapped here from the shape the
// dialer wraps them in.
func TestDialProblem(t *testing.T) {
	dialErr := func(err error) error {
		return &net.OpError{Op: "dial", Net: "tcp", Err: err}
	}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"refused", dialErr(&os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}), problemConnectionRefused},
		{"host unreachable", dialErr(&os.SyscallError{Syscall: "connect", Err: syscall.EHOSTUNREACH}), problemUnreachable},
		{"network unreachable", dialErr(&os.SyscallError{Syscall: "connect", Err: syscall.ENETUNREACH}), problemUnreachable},
		{"unresolvable host", dialErr(&net.DNSError{Err: "no such host", IsNotFound: true}), problemHostNotFound},
		{"malformed address", &net.AddrError{Err: "missing port in address"}, problemInvalidAddress},
		// Three producers, one label. Which one returns is a race, so matching
		// only the context deadline made the diagnosis intermittent.
		{"context deadline", dialErr(context.DeadlineExceeded), problemTimedOut},
		{"socket deadline", dialErr(os.ErrDeadlineExceeded), problemTimedOut},
		{"OS gave up", dialErr(&os.SyscallError{Syscall: "connect", Err: syscall.ETIMEDOUT}), problemTimedOut},
		{"cancelled", dialErr(context.Canceled), problemCancelled},
		// Reached by EACCES and ECONNRESET among others. The default names no
		// cause because it has not established one.
		{"anything else", dialErr(&os.SyscallError{Syscall: "connect", Err: syscall.EACCES}), problemConnectionFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dialProblem(tt.err); got != tt.want {
				t.Errorf("dialProblem(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func openListener(t *testing.T) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0") // Start a listener on a random available port
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	t.Cleanup(func() {
		_ = listener.Close()
	})
	return listener
}
