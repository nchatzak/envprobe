package probe

import (
	"os"
	"regexp"
	"testing"
)

const envVar = "ENVPROBE_TEST_ENV_VAR"

func TestEnvCheckRun(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		unset       bool
		matches     string
		wantFound   bool
		wantProblem string
	}{
		{name: "set, no pattern", value: "/opt/jdk", wantFound: true},
		{name: "set empty, no pattern", value: "", wantFound: true},
		{name: "unset", unset: true, wantProblem: problemNotSet},
		{name: "unset with a pattern", unset: true, matches: ".", wantProblem: problemNotSet},
		{name: "pattern matches", value: "/opt/jdk", matches: `^/.*jdk`, wantFound: true},
		{name: "pattern misses", value: "/opt/nope", matches: `^/.*jdk`, wantProblem: problemDoesNotMatch},
		{name: "empty against a pattern", value: "", matches: ".", wantProblem: problemSetButEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv registers the restore; Unsetenv then gives us an
			// absent variable.
			t.Setenv(envVar, tt.value)
			if tt.unset {
				_ = os.Unsetenv(envVar)
			}
			const checkName = "Java home"
			check := envCheck{name: checkName, target: envVar}
			if tt.matches != "" {
				check.matches = regexp.MustCompile(tt.matches)
			}

			got := check.Run(t.Context())
			if got.Name != checkName {
				t.Errorf("Name = %q, want %q", got.Name, checkName)
			}
			if got.Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", got.Found, tt.wantFound)
			}
			if got.Problem != tt.wantProblem {
				t.Errorf("Problem = %v, want %v", got.Problem, tt.wantProblem)
			}
		})
	}
}

func mustEnvCheck(t *testing.T, name string, with map[string]any) envCheck {
	t.Helper()
	check, err := newEnvCheck(name, with)
	if err != nil {
		t.Fatalf("newEnvCheck(%q) error = %v", name, err)
	}
	return check.(envCheck)
}

func TestNewEnvCheck(t *testing.T) {
	t.Run("target defaults to name", func(t *testing.T) {
		env := mustEnvCheck(t, "AWS_PROFILE", nil)
		if env.target != "AWS_PROFILE" {
			t.Errorf("target = %q, want %q", env.target, "AWS_PROFILE")
		}
		if env.matches != nil {
			t.Errorf("matches = %v, want nil", env.matches)
		}
	})

	t.Run("matches is compiled", func(t *testing.T) {
		env := mustEnvCheck(t, "Java home", map[string]any{"matches": "^/.*jdk"})
		if env.matches == nil {
			t.Fatal("matches = nil, want a compiled pattern")
		}
		if got := env.matches.String(); got != "^/.*jdk" {
			t.Errorf("matches = %q, want %q", got, "^/.*jdk")
		}
	})

	t.Run("invalid pattern is rejected", func(t *testing.T) {
		if _, err := newEnvCheck("Java home", map[string]any{"matches": "["}); err == nil {
			t.Error("newEnvCheck() error = nil, want an error")
		}
	})

	t.Run("unknown key is rejected", func(t *testing.T) {
		if _, err := newEnvCheck("Java home", map[string]any{"macthes": "."}); err == nil {
			t.Error("newEnvCheck() error = nil, want an error")
		}
	})
}
