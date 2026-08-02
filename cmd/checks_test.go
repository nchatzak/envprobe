package cmd

// No Parallel in this file!! Every test redirects the config search path with
// t.Chdir and t.Setenv, which mutate process-wide state. t.Setenv panics
// outright if the test has called t.Parallel.

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nchatzak/envprobe/internal/probe"
)

// The name loadConfig searches for. Written to the working directory that
// isolate creates, so tests can rebuild the path loadConfig will report.
const testConfigFileName = "envprobe.yaml"

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

// wantConfigPath is the path loadConfig reports for a file written to the
// working directory isolate created. The full path, not its base: isolate points
// cwd and home at different dirs, so comparing all of it is what tells the two
// lookups apart.
func wantConfigPath(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(cwd, testConfigFileName)
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

// The two cases the old fallback conflated, now split. No config file means
// the user never said what to check, so reporting a pass would be a lie.
func TestConfiguredChecksNoConfig(t *testing.T) {
	isolate(t)

	checks, _, err := configuredChecks()
	if !errors.Is(err, errNoConfig) {
		t.Fatalf("configuredChecks() = %#v, %v; want errNoConfig", checks, err)
	}
}

// An empty list is an explicit choice to check nothing: no error, no checks.
func TestConfiguredChecksEmptyList(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, "checks: []\n")

	checks, _, err := configuredChecks()
	if err != nil {
		t.Fatalf("configuredChecks() error = %v", err)
	}
	if len(checks) != 0 {
		t.Errorf("configuredChecks() = %#v, want no checks", checks)
	}
}

func TestConfiguredChecksFromConfig(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, twoCheckConfig)

	checks, path, err := configuredChecks()
	if err != nil {
		t.Fatalf("configuredChecks() error = %v", err)
	}

	if want := wantConfigPath(t); path != want {
		t.Errorf("configuredChecks() path = %q, want %q", path, want)
	}

	want := []string{"alpha", "beta"}
	if got := checkNames(t, checks); !slices.Equal(got, want) {
		t.Errorf("check names = %v, want %v", got, want)
	}
}

// Viper never gets far enough to hand us a checks list, so this asserts on its
// message: the failure belongs to the YAML parser and has no sentinel of ours.
func TestConfiguredChecksMalformedYAML(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, "checks: [\n")

	checks, _, err := configuredChecks()
	if err == nil {
		// Don't call checkNames here: it runs the checks, and this is the
		// branch where we have no idea what got built.
		t.Fatalf("configuredChecks() = %#v, want an error", checks)
	}
	if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("error = %q, want it to name the parse failure", err)
	}

	// The source line cannot carry the path here: the read failed, so
	// configuredChecks has no viper to report one from. It is prefixed onto the
	// error instead, which reads correctly because a parse error is one line.
	if want := wantConfigPath(t); !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to name %q", err, want)
	}
}

// Past the parser, LoadChecks rejects the entry, so this matches the sentinel
// rather than the prose wrapped around it by CheckError.
func TestConfiguredChecksUnknownType(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, "checks:\n  - name: alpha\n    type: nonsense\n")

	checks, path, err := configuredChecks()
	if !errors.Is(err, probe.ErrUnknownType) {
		t.Fatalf("configuredChecks() = %#v, %v; want ErrUnknownType", checks, err)
	}

	// A file was read, so the caller can still name it. errors.Is cannot see
	// this: the error is identical either way.
	if path == "" {
		t.Error("configuredChecks() path = \"\", want the file that failed to build")
	}
}
