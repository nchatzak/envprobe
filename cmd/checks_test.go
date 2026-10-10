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

// Two checks that are fine apart from a key envprobe does not know, one each.
const twoUnknownKeysConfig = `
checks:
  - name: alpha
    type: binary
    hint: install alpha
  - name: beta
    type: binary
    tip: install beta
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

// Unknown keys on the checks themselves, which viper ignored until StrictKeys.
// Two of them, on different entries: this decode covers the whole list at once,
// so one run has to name both. The assertion is the keys, since the prose
// around them is mapstructure's and carries no sentinel to match on.
func TestConfiguredChecksUnknownKeys(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, twoUnknownKeysConfig)

	checks, _, err := configuredChecks()
	if err == nil {
		t.Fatalf("configuredChecks() = %#v, want an error", checks)
	}
	for _, key := range []string{"hint", "tip"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error = %q, want it to name %q", err, key)
		}
	}
}

// A misspelled name is the case the strict decode is worth having: it used to
// reach LoadChecks as an entry with no name at all and report ErrNameRequired,
// which sends the reader to the wrong line.
func TestConfiguredChecksMisspelledName(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, "checks:\n  - nmae: alpha\n    type: binary\n")

	checks, _, err := configuredChecks()
	if err == nil {
		t.Fatalf("configuredChecks() = %#v, want an error", checks)
	}
	if !strings.Contains(err.Error(), "nmae") {
		t.Errorf("error = %q, want it to name the unknown key", err)
	}
	if errors.Is(err, probe.ErrNameRequired) {
		t.Errorf("error = %q, want the typo reported rather than a missing name", err)
	}
}

// Past the parser, LoadChecks rejects the entry, so this matches the sentinel
// rather than the prose wrapped around it by CheckError.
func TestConfiguredChecksUnknownType(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, "checks:\n  - name: alpha\n    type: nonsense\n")

	checks, path, err := configuredChecks()
	if !errors.Is(err, probe.ErrUnknownCheckType) {
		t.Fatalf("configuredChecks() = %#v, %v; want ErrUnknownCheckType", checks, err)
	}

	// A file was read, so the caller can still name it. errors.Is cannot see
	// this: the error is identical either way.
	if path == "" {
		t.Error("configuredChecks() path = \"\", want the file that failed to build")
	}
}

// A top-level key that is not checks: the file may define something other than
// what its author meant, so nothing under it is trusted. The assertions are the
// key names, which is what a reader has to act on; the sentinel carries the
// rest.
func TestConfiguredChecksUnknownTopLevelKey(t *testing.T) {
	tests := []struct {
		name string
		// config is written to the working directory isolate creates.
		config string
		// named substrings, in the order the error must report them.
		named []string
		// substrings the error must not contain.
		absent []string
	}{
		{
			name:   "misspelled checks",
			config: "chekcs:\n  - name: alpha\n    type: binary\n",
			named:  []string{`"chekcs"`},
		},
		{
			name:   "stray key beside checks",
			config: "checks: []\nverbose: true\n",
			named:  []string{`"verbose"`},
			absent: []string{`"checks"`},
		},
		{
			name:   "two stray keys, sorted",
			config: "checks: []\nzebra: 1\nalpha: 2\n",
			named:  []string{`"alpha"`, `"zebra"`},
		},
		{
			name:   "nested key names its root",
			config: "checks: []\ndefaults:\n  timeout: 5\n",
			named:  []string{`"defaults"`},
			absent: []string{"timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			writeFile(t, testConfigFileName, tt.config)

			checks, path, err := configuredChecks()
			if !errors.Is(err, errUnknownTopLevelKey) {
				t.Fatalf("configuredChecks() = %#v, %v; want errUnknownTopLevelKey", checks, err)
			}

			// A file was read, so the caller can still name it.
			if path == "" {
				t.Error("configuredChecks() path = \"\", want the file that was rejected")
			}

			assertNamedInOrder(t, err.Error(), tt.named)
			for _, absent := range tt.absent {
				if strings.Contains(err.Error(), absent) {
					t.Errorf("error = %q, want it not to contain %q", err, absent)
				}
			}
		})
	}
}

// assertNamedInOrder checks that every substring appears, and in this order.
// The order is the assertion that AllKeys' map order was sorted away.
func assertNamedInOrder(t *testing.T, got string, want []string) {
	t.Helper()
	at := 0
	for _, key := range want {
		i := strings.Index(got[at:], key)
		if i < 0 {
			t.Errorf("error = %q, want %q after position %d", got, key, at)
			return
		}
		at += i + len(key)
	}
}

// No checks key at all reports the same way a misspelled one does: a file that
// says nothing about what to check is not a machine that passed.
func TestConfiguredChecksNoChecksKey(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{"empty file", ""},
		{"comments only", "# nothing here yet\n# not even a key\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolate(t)
			writeFile(t, testConfigFileName, tt.config)

			checks, _, err := configuredChecks()
			if !errors.Is(err, errChecksKeyMissing) {
				t.Fatalf("configuredChecks() = %#v, %v; want errChecksKeyMissing", checks, err)
			}
		})
	}
}

// The key is there with nothing under it, which is the same deliberate choice
// as an empty list. This is also what catches a viper upgrade that stops
// reporting a null-valued key from AllKeys.
func TestConfiguredChecksNullChecksKey(t *testing.T) {
	isolate(t)
	writeFile(t, testConfigFileName, "checks:\n")

	checks, _, err := configuredChecks()
	if err != nil {
		t.Fatalf("configuredChecks() error = %v", err)
	}
	if len(checks) != 0 {
		t.Errorf("configuredChecks() = %#v, want no checks", checks)
	}
}
