package doctor

import (
	"context"
	"net"
	"testing"
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
	})

	t.Run("Port closed", func(t *testing.T) {
		listener := openListener(t)
		target := listener.Addr().String()
		listener.Close() // Close the listener to simulate a closed port
		check := portCheck{name: "test-port-closed", target: target}

		result := check.Run(t.Context())
		if result.Found {
			t.Errorf("Expected target %s to not be accessible", target)
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
	})

	t.Run("Malformed target", func(t *testing.T) {
		check := portCheck{name: "test-port-malformed", target: "not-a-valid-target"}

		result := check.Run(t.Context())
		if result.Found {
			t.Errorf("Expected target %s to not be accessible", check.target)
		}
	})
}

func openListener(t *testing.T) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0") // Start a listener on a random available port
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	t.Cleanup(func() {
		listener.Close()
	})
	return listener
}
