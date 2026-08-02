package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nchatzak/envprobe/internal/probe"
)

func TestConfigExample(t *testing.T) {
	stdout, _, err := execute(t, "config", "example")
	if err != nil {
		t.Fatalf("config example: %v", err)
	}
	if !strings.HasPrefix(stdout, "# envprobe configuration") {
		t.Errorf("unexpected output: %q", stdout)
	}
}

func TestConfigInit(t *testing.T) {
	path := tempPath(t)

	stdout, _, err := execute(t, "config", "init", "-o", path)
	if err != nil {
		t.Fatalf("config init: %v", err)
	}
	if !strings.Contains(stdout, "wrote") {
		t.Errorf("expected a confirmation, got %q", stdout)
	}

	got := readFile(t, path)
	if got != probe.ExampleConfig {
		t.Error("written file does not match the embedded template")
	}
}

func TestConfigInitExisting(t *testing.T) {
	path := tempPath(t)
	writeFile(t, path, "sentinel")

	stdout, stderr, err := execute(t, "config", "init", "-o", path)
	if err == nil {
		t.Fatal("expected an error for an existing file")
	}
	assertNoUsage(t, "config init", stdout, stderr)
	if got := readFile(t, path); got != "sentinel" {
		t.Errorf("file was clobbered: %q", got)
	}
}

func TestConfigInitForce(t *testing.T) {
	path := tempPath(t)
	writeFile(t, path, "sentinel")

	if _, _, err := execute(t, "config", "init", "-o", path, "--force"); err != nil {
		t.Fatalf("config init --force: %v", err)
	}
	if readFile(t, path) != probe.ExampleConfig {
		t.Error("--force did not overwrite")
	}
}

func TestConfigInitRejectsArgs(t *testing.T) {
	stdout, stderr, err := execute(t, "config", "init", "extraParam")
	if err == nil {
		t.Fatalf("expected an error from unexpected argument")
	}
	// The mirror of assertNoUsage: an Args violation is misuse, and misuse
	// fails before RunE can set SilenceUsage, so the usage block must appear.
	if out := stdout + stderr; !strings.Contains(out, "Usage:") {
		t.Errorf("config init did not print usage for an args violation:\n%s", out)
	}
}

func TestConfigValidate(t *testing.T) {
	path := tempPath(t)
	writeFile(t, path, probe.ExampleConfig)

	stdout, _, err := execute(t, "config", "validate", path)
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if !strings.Contains(stdout, "checks OK") {
		t.Fatalf("expected a summary, got %q", stdout)
	}
}

func TestConfigValidateBadType(t *testing.T) {
	path := tempPath(t)
	writeFile(t, path, "checks:\n  - name: x\n    type: bogus\n")

	stdout, stderr, err := execute(t, "config", "validate", path)
	if err == nil {
		t.Fatalf("expected an error, got none")
	}
	if !errors.Is(err, probe.ErrUnknownType) {
		t.Errorf("expected %q, got %q", probe.ErrUnknownType, err)
	}
	// Named above the error, not inside it: LoadChecks joins its failures, so a
	// prefix would label only the first entry of the set.
	if want := "using " + path + "\n"; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, want)
	}
	assertNoUsage(t, "config validate", stdout, stderr)
}

// The explicit-path loader prefixes the file onto viper's parse error, so this
// failure reads the same whether the file was searched for or named on the
// command line.
func TestConfigValidateMalformedYAML(t *testing.T) {
	path := tempPath(t)
	writeFile(t, path, "checks: [\n")

	stdout, stderr, err := execute(t, "config", "validate", path)
	if err == nil {
		t.Fatal("expected an error, got none")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error = %q, want it to name %q", err, path)
	}
	assertNoUsage(t, "config validate", stdout, stderr)
}

func TestConfigValidateNoChecks(t *testing.T) {
	path := tempPath(t)
	writeFile(t, path, "checks: []\n")

	_, stderr, err := execute(t, "config", "validate", path)
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if !strings.Contains(stderr, "no checks") {
		t.Fatalf("expected a no checks error, got %q", stderr)
	}
}

func TestConfigValidateFindsConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, testConfigFileName)
	writeFile(t, path, probe.ExampleConfig)

	stdout, _, err := execute(t, "config", "validate")
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if !strings.Contains(stdout, "checks OK") {
		t.Fatalf("expected a summary, got %q", stdout)
	}
}

// No config anywhere on the search path is an error, not a warning: validate
// and doctor must agree on whether this environment is configured, and nothing
// was. Both search locations are redirected at empty temp dirs so the result
// cannot depend on the machine running the test.
func TestConfigValidateNoConfigFound(t *testing.T) {
	isolate(t)

	stdout, stderr, err := execute(t, "config", "validate")
	if !errors.Is(err, errNoConfig) {
		t.Fatalf("config validate error = %v, want errNoConfig", err)
	}
	assertNoUsage(t, "config validate", stdout, stderr)
}

// assertNoUsage checks that a RunE failure stayed quiet. Both streams are
// concatenated because which one carries the usage block is an artifact of the
// SetOut call in execute: cobra routes it through OutOrStderr, so the real
// binary writes it to stderr.
func assertNoUsage(t *testing.T, cmd, stdout, stderr string) {
	t.Helper()
	if out := stdout + stderr; strings.Contains(out, "Usage:") {
		t.Errorf("%s printed usage for a runtime failure:\n%s", cmd, out)
	}
}

func execute(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), testConfigFileName)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
