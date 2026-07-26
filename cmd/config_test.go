package cmd

import (
	"bytes"
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

	if _, _, err := execute(t, "config", "init", "-o", path); err == nil {
		t.Fatal("expected an error for an existing file")
	}
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
	_, _, err := execute(t, "config", "init", "extraParam")
	if err == nil {
		t.Fatalf("expected an error from unexpected argument")
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

	_, stderr, err := execute(t, "config", "validate", path)
	if err == nil {
		t.Fatalf("expected an error, got none")
	}
	if !strings.Contains(stderr, "unknown check type") {
		t.Fatalf("expected a type error, got %q", stderr)
	}
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
	path := filepath.Join(dir, "envprobe.yaml")
	writeFile(t, path, probe.ExampleConfig)

	stdout, _, err := execute(t, "config", "validate")
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if !strings.Contains(stdout, "checks OK") {
		t.Fatalf("expected a summary, got %q", stdout)
	}
}

// No config anywhere on the search path is a warning, not an error — doctor
// still has its defaults. Both search locations are redirected at empty temp
// dirs so the result cannot depend on the machine running the test.
func TestConfigValidateNoConfigFound(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())

	_, stderr, err := execute(t, "config", "validate")
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if !strings.Contains(stderr, "no config file found") {
		t.Errorf("expected a warning, got %q", stderr)
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
	return filepath.Join(t.TempDir(), "envprobe.yaml")
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
