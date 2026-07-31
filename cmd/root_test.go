package cmd

import (
	"slices"
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
