package probe

import (
	"errors"
	"net"
	"testing"
)

// Built directly rather than through LoadChecks: this asserts the formatter,
// not the loader. errors.Is never calls Error(), so nothing else covers it.
func TestCheckErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *CheckError
		want string
	}{
		{
			name: "with name",
			err:  &CheckError{Index: 3, Name: "pg", Err: ErrUnknownType},
			want: `checks[3] "pg": unknown check type`,
		},
		{
			// No name to quote — `checks[0] "":` would read as an empty label.
			name: "without name",
			err:  &CheckError{Index: 0, Err: ErrNameRequired},
			want: "checks[0]: name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The wrapped case is the one callers rely on: dialProblem sees *net.DNSError
// through the *net.OpError the dialer wraps it in, never on its own.
func TestIsType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"direct match", &net.DNSError{Err: "no such host"}, true},
		{"wrapped match", &net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host"}}, true},
		{"different type", &net.AddrError{Err: "missing port"}, false},
		{"no cause", errors.New("plain"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isType[*net.DNSError](tt.err); got != tt.want {
				t.Errorf("isType[*net.DNSError](%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
