package probe

import (
	"regexp"
	"testing"
)

func TestCheckTool(t *testing.T) {
	tests := []struct {
		name string
		tool string
		want bool
	}{
		{"go is on PATH", "go", true},
		{"non-existent tool is not on PATH", "nonexistenttool", false},
		{"empty string is not on PATH", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkTool(tt.tool); got.Found != tt.want {
				t.Errorf("checkTool(%q) = %v, want %v", tt.tool, got.Found, tt.want)
			}
		})
	}
}

func TestBinaryCheckRun(t *testing.T) {
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
