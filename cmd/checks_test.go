package cmd

// No Parallel in this file!! Every test redirects the config search path with
// t.Chdir and t.Setenv, which mutate process-wide state. t.Setenv panics
// outright if the test has called t.Parallel.

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nchatzak/envprobe/internal/probe"
)

// Two binary checks aimed at targets that cannot exist, so Run is a failed
// exec.LookPath and never spawns a subprocess.
const twoCheckConfig = `
checks:
  - name: alpha
    type: binary
    with:
      target: envprobe-no-such-binary
  - name: beta
    type: binary
    with:
      target: envprobe-also-missing
`

// isolate points both config search roots at empty temp dirs, so the result
// cannot depend on the machine running the test. Two separate dirs on purpose:
// sharing one would put a file written to the working directory on the home
// path too, hiding which lookup actually succeeded.
//
// Both home variables are set because loadConfig resolves the home directory
// with os.UserHomeDir, which reads USERPROFILE on Windows and HOME elsewhere.
// Setting only HOME would leave the real home dir on the search path there.
func isolate(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Chdir(t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// checkNames runs the checks and collects the names they report. probe.Check
// has only Run, and the concrete types are unexported in another package, so
// running them is the only way to observe what was built.
func checkNames(t *testing.T, checks []probe.Check) []string {
	t.Helper()
	results := probe.RunAll(t.Context(), checks)
	names := make([]string, len(results))
	for i, result := range results {
		names[i] = result.Name
	}
	return names
}

// Asserts provenance, not content: that the fallback branch handed back
// exactly what DefaultChecks builds. Whether those defaults are the right
// checks is probe.TestDefaultChecks's job, and it asserts them field by field.
// Comparing the values also keeps this test hermetic — reading their names
// would mean running them, which shells out to git, java, go and docker.
func TestConfiguredChecksFallsBackToDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config string // "" means: write no config file at all
	}{
		{"no config found", ""},
		{"config with an empty check list", "checks: []\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			if tt.config != "" {
				writeFile(t, "envprobe.yaml", tt.config)
			}

			checks, err := configuredChecks()
			if err != nil {
				t.Fatalf("configuredChecks() error = %v", err)
			}

			if want := probe.DefaultChecks(); !reflect.DeepEqual(checks, want) {
				t.Errorf("configuredChecks() = %#v, want the defaults %#v", checks, want)
			}
		})
	}
}

func TestConfiguredChecksFromConfig(t *testing.T) {
	isolate(t)
	writeFile(t, "envprobe.yaml", twoCheckConfig)

	checks, err := configuredChecks()
	if err != nil {
		t.Fatalf("configuredChecks() error = %v", err)
	}

	want := []string{"alpha", "beta"}
	if got := checkNames(t, checks); !slices.Equal(got, want) {
		t.Errorf("check names = %v, want %v", got, want)
	}
}

func TestConfiguredChecksErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string // substring the error must contain
	}{
		{
			name:    "malformed yaml",
			config:  "checks: [\n",
			wantErr: "parsing config",
		},
		{
			// TODO(ch14): assert with errors.Is once LoadChecks has sentinels.
			name:    "unknown check type",
			config:  "checks:\n  - name: alpha\n    type: nonsense\n",
			wantErr: "unknown check type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			writeFile(t, "envprobe.yaml", tt.config)

			checks, err := configuredChecks()
			if err == nil {
				// Don't call checkNames here: it runs the checks, and this is
				// the branch where we have no idea what got built.
				t.Fatalf("configuredChecks() = %#v, want an error", checks)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
