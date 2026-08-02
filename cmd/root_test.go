package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestRootRegistersSubcommands(t *testing.T) {
	t.Parallel()
	var got []string
	for _, c := range newRootCmd().Commands() {
		got = append(got, c.Name())
	}

	for _, want := range []string{"doctor", "config"} {
		if !slices.Contains(got, want) {
			t.Errorf("newRootCmd() has no %q subcommand, got %v", want, got)
		}
	}
}

func TestRootPrintUsage(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"doctor", "--nope"})

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute() did not fail")
	}

	// Concatenated on purpose: usage lands in outBuf only because SetOut was
	// called. Cobra routes it through OutOrStderr, so the real binary prints
	// it to stderr — asserting on outBuf alone would document the opposite.
	if out := outBuf.String() + errBuf.String(); !strings.Contains(out, "Usage:") {
		t.Errorf("usage not printed for unknown flag; got %q", out)
	}
	if !strings.Contains(errBuf.String(), "unknown flag") {
		t.Errorf("error not printed for unknown flag; got %q", errBuf.String())
	}
}

func TestRootVersionFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{name: "long flag", args: []string{"--version"}},
		{name: "shorthand", args: []string{"-v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := newRootCmd()
			cmd.SetArgs(tt.args)

			var outBuf bytes.Buffer
			cmd.SetOut(&outBuf)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() = %v, want nil", err)
			}

			if got := outBuf.String(); !strings.Contains(got, version) {
				t.Errorf("version output = %q, want it to contain %q", got, version)
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "no error", err: nil, want: 0},
		{name: "checks failed", err: checksFailedError{failed: 1, total: 3}, want: 1},
		{name: "no checks error", err: errNoChecks, want: 2},
		{name: "no config error", err: errNoConfig, want: 2},
		{name: "other error", err: errors.New("boom"), want: 2},
		{name: "wrapper error", err: fmt.Errorf("wrapped: %w", checksFailedError{failed: 2, total: 2}), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := exitCode(tt.err); got != tt.want {
				t.Errorf("exitCode() = %v, want %v", got, tt.want)
			}
		})
	}
}
