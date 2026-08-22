package probe

import (
	"regexp"
	"testing"
)

func TestBinaryCheckRun(t *testing.T) {
	atLeast120 := mustConstraint(t, ">= 1.20.0")
	below120 := mustConstraint(t, "< 1.20.0")
	tests := []struct {
		name          string      // the subtest label, not the check's name
		check         binaryCheck // the input
		wantName      string
		wantFound     bool
		wantVersionRe string
		wantProblem   string
	}{
		{
			name:          "version parsed",
			check:         binaryCheck{name: "go", target: "go", versionArgs: []string{"version"}},
			wantName:      "go",
			wantFound:     true,
			wantVersionRe: `^\d+\.\d+`,
		},
		// A binary that is not on PATH reports Found false and nothing else:
		// the status already says what went wrong.
		{
			name:          "missing target",
			check:         binaryCheck{name: "Nonexistent Tool", target: "nonexistenttool"},
			wantName:      "Nonexistent Tool",
			wantFound:     false,
			wantVersionRe: `^$`,
		},
		{
			name:          "version command fails",
			check:         binaryCheck{name: "go", target: "go", versionArgs: []string{"bogus-subcommand"}},
			wantName:      "go",
			wantFound:     true,
			wantVersionRe: `^$`,
			wantProblem:   problemVersionCommandFailed,
		},
		{
			name:          "version constraint succeeds",
			check:         binaryCheck{name: "go", target: "go", versionArgs: []string{"version"}, versionConstraint: atLeast120},
			wantName:      "go",
			wantFound:     true,
			wantVersionRe: `^\d+\.\d+\.\d+$`,
		},
		{
			name:          "version constraint fails",
			check:         binaryCheck{name: "go", target: "go", versionArgs: []string{"version"}, versionConstraint: below120},
			wantName:      "go",
			wantFound:     false,
			wantVersionRe: `^\d+\.\d+\.\d+$`,
			wantProblem:   problemVersionDoesNotMatch,
		},
		{
			name:          "version cannot be compared",
			check:         binaryCheck{name: "go", target: "go", versionArgs: []string{"env", "GOOS"}, versionConstraint: atLeast120},
			wantName:      "go",
			wantFound:     false,
			wantVersionRe: `^[a-z]+$`,
			wantProblem:   problemCannotCompareVersion,
		},
		{
			name:          "broken command with constraint",
			check:         binaryCheck{name: "go", target: "go", versionArgs: []string{"bogus-subcommand"}, versionConstraint: atLeast120},
			wantName:      "go",
			wantFound:     false,
			wantVersionRe: `^$`,
			wantProblem:   problemVersionCommandFailed,
		},
		{
			name:          "no constraint, unreadable output",
			check:         binaryCheck{name: "go", target: "go", versionArgs: []string{"env", "GOOS"}},
			wantName:      "go",
			wantFound:     true,
			wantVersionRe: `^[a-z]+$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got := tt.check.Run(t.Context())
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}

			if tt.wantFound != got.Found {
				t.Errorf("Found = %v, want %v", got.Found, tt.wantFound)
			}

			if !regexp.MustCompile(tt.wantVersionRe).MatchString(got.Version) {
				t.Errorf("Version = %q, want match %q", got.Version, tt.wantVersionRe)
			}

			if got.Problem != tt.wantProblem {
				t.Errorf("Problem = %q, want %q", got.Problem, tt.wantProblem)
			}
		})
	}
}
