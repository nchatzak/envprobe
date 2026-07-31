package cmd

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestRootRegistersSubcommands(t *testing.T) {
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
