package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/nchatzak/envprobe/internal/probe"
)

const fakeConfigPath = "test.yaml"

type fakeCheck struct {
	result probe.Result
}

func (f fakeCheck) Run(context.Context) probe.Result {
	return f.result
}

func runDoctor(t *testing.T, load checkLoader, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	cmd := newDoctorCmd(load)

	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	// Must stay non-nil: cobra falls back to os.Args[1:] otherwise, which
	// under `go test` is a list of -test.* flags.
	cmd.SetArgs(append([]string{}, args...))

	err = cmd.ExecuteContext(t.Context())
	return out.String(), errOut.String(), err
}

func staticLoader(checks ...probe.Check) checkLoader {
	return func() ([]probe.Check, string, error) {
		return checks, fakeConfigPath, nil
	}
}

// source is empty when no config file was found and populated when one was read
// but could not be built -- the two shapes runDoctorCmd distinguishes.
func failingLoader(source string, err error) checkLoader {
	return func() ([]probe.Check, string, error) {
		return nil, source, err
	}
}

func found(name string) fakeCheck {
	return fakeCheck{
		result: probe.Result{Name: name, Found: true},
	}
}

func missing(name string) fakeCheck {
	return fakeCheck{
		result: probe.Result{Name: name, Found: false},
	}
}

func TestDoctorOutput(t *testing.T) {
	t.Parallel()
	out, _, err := runDoctor(t, staticLoader(found("git"), missing("docker")))
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"git", "docker"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing %q:\n%s", name, out)
		}
	}
}

// The config file doctor read is named on stderr, so a CI log records what was
// checked. HasPrefix rather than Contains: it pins the wording and that nothing
// precedes it. Note what neither can see — stdout and stderr are separate
// buffers here, so "printed before the results" is not observable from stderr.
func TestDoctorReportsConfigSource(t *testing.T) {
	t.Parallel()
	stdout, stderr, err := runDoctor(t, staticLoader(found("git")))
	if err != nil {
		t.Fatal(err)
	}

	want := "using " + fakeConfigPath + "\n"
	if !strings.HasPrefix(stderr, want) {
		t.Errorf("stderr = %q, want it to start with %q", stderr, want)
	}

	// stdout carries results only: the line must not reach a --json consumer.
	if strings.Contains(stdout, fakeConfigPath) {
		t.Errorf("config path leaked into stdout:\n%s", stdout)
	}
}

// Zero checks puts the source line and the warning on the same stream, which is
// the one place the order is real rather than an artifact of two buffers. A
// reader has to see which file produced the warning before the warning itself.
func TestDoctorReportsConfigSourceBeforeWarning(t *testing.T) {
	t.Parallel()
	_, stderr, err := runDoctor(t, staticLoader())
	if err != nil {
		t.Fatal(err)
	}

	source, warning := strings.Index(stderr, "using "), strings.Index(stderr, "warning:")
	if source < 0 || warning < 0 || source > warning {
		t.Errorf("stderr = %q, want the source line before the warning", stderr)
	}
}

func TestDoctorJSON(t *testing.T) {
	t.Parallel()

	// Declared here rather than reusing probe's own DTO: sharing the struct
	// tags with the producer would make a renamed tag pass on both sides.
	type jsonRow struct {
		Name  string `json:"name"`
		Found bool   `json:"found"`
	}

	out, _, err := runDoctor(t, staticLoader(found("git"), missing("docker")), "--json")
	if err != nil {
		t.Fatal(err)
	}

	var got []jsonRow
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	want := []jsonRow{{"git", true}, {"docker", false}}
	if !slices.Equal(got, want) {
		t.Errorf("decoded %v, want %v", got, want)
	}
}

// The count is a diagnostic, so it goes to stderr under both formats. Asserting
// its absence from stdout is the half that matters: on stdout it would be a
// trailing line after the JSON array, and --json would stop parsing.
func TestDoctorSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{"text", nil},
		{"json", []string{"--json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, err := runDoctor(t, staticLoader(found("git"), missing("docker")), tt.args...)
			if err != nil {
				t.Fatal(err)
			}

			const want = "1 of 2 checks passed"
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, want)
			}
			if strings.Contains(stdout, want) {
				t.Errorf("summary leaked into stdout:\n%s", stdout)
			}
		})
	}
}

// Zero checks is valid config, so this exits 0 — the warning on stderr is the
// only evidence doctor ran at all, since the table renderer emits nothing.
func TestDoctorNoChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{"text", nil, ""},
		{"json", []string{"--json"}, "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, err := runDoctor(t, staticLoader(), tt.args...)
			if err != nil {
				t.Fatalf("doctor with no checks: %v", err)
			}
			if got := strings.TrimSpace(stdout); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if !strings.Contains(stderr, "no checks") {
				t.Errorf("stderr = %q, want a no-checks warning", stderr)
			}
		})
	}
}

// --ci turns zero checks into a hard failure, reported before RunAll: stdout
// stays empty under both renderers, where TestDoctorNoChecks gets an empty
// table and []. Asserts the sentinel rather than its prose, and that the
// warning is suppressed so the run reports the condition once.
func TestDoctorCINoChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{"text", []string{"--ci"}},
		{"json", []string{"--ci", "--json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, err := runDoctor(t, staticLoader(), tt.args...)
			if !errors.Is(err, errNoChecks) {
				t.Fatalf("doctor %s with no checks: err = %v, want errNoChecks",
					strings.Join(tt.args, " "), err)
			}
			if stdout != "" {
				t.Errorf("expected no output, got:\n%s", stdout)
			}
			if strings.Contains(stderr, "warning:") {
				t.Errorf("reported twice, warning and error:\n%s", stderr)
			}
			assertNoUsage(t, "doctor "+strings.Join(tt.args, " "), stdout, stderr)
		})
	}
}

func TestDoctorCI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		checks  []probe.Check
		wantErr bool
	}{
		{"all pass", []probe.Check{found("git"), found("docker")}, false},
		{"one fails", []probe.Check{found("git"), missing("docker")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, err := runDoctor(t, staticLoader(tt.checks...), "--ci")
			// A failing check is a runtime failure, not misuse: no usage block.
			assertNoUsage(t, "doctor --ci", stdout, stderr)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, errChecksFailed) {
				t.Fatalf("error = %v (%T), want errChecksFailed", err, err)
			}
		})
	}
}

// The whole diagnostic stream of a failing --ci run, so a tally added back to
// the error fails here however it is worded. Counting substrings instead would
// only catch the wording that was removed.
func TestDoctorCIStderr(t *testing.T) {
	t.Parallel()
	_, stderr, err := runDoctor(t, staticLoader(found("git"), missing("docker")), "--ci")
	if !errors.Is(err, errChecksFailed) {
		t.Fatalf("error = %v, want errChecksFailed", err)
	}

	want := "using " + fakeConfigPath + "\n1 of 2 checks passed\nError: checks failed\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

// No config file was found, so there is nothing to name: stderr carries the
// error and nothing else.
func TestDoctorLoaderError(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	out, stderr, err := runDoctor(t, failingLoader("", errBoom))
	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
	if out != "" {
		t.Errorf("expected no output, got:\n%s", out)
	}
	if strings.Contains(stderr, "using") {
		t.Errorf("named a config file when none was read:\n%s", stderr)
	}
}

// A file was read but could not be built. The failure is about that file's
// contents, so the run names it before reporting the error.
func TestDoctorNamesConfigThatFailedToBuild(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	_, stderr, err := runDoctor(t, failingLoader(fakeConfigPath, errBoom))
	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}

	want := "using " + fakeConfigPath + "\n"
	if !strings.HasPrefix(stderr, want) {
		t.Errorf("stderr = %q, want it to start with %q", stderr, want)
	}
}

var errWrite = errors.New("write failed")

// errWriter stands in for a closed pipe or a full disk, so that the render
// error has somewhere to come from.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errWrite }

// Covers the wiring rather than the renderers: probe already tests that both
// return the writer's error, this asserts runDoctorCmd propagates it.
func TestDoctorRenderError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{"text", nil},
		{"json", []string{"--json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := newDoctorCmd(staticLoader(found("git")))
			cmd.SetOut(errWriter{})
			cmd.SetErr(&bytes.Buffer{})
			// Must stay non-nil: cobra falls back to os.Args[1:] otherwise,
			// which under `go test` is a list of -test.* flags.
			cmd.SetArgs(append([]string{}, tt.args...))

			err := cmd.ExecuteContext(t.Context())
			if !errors.Is(err, errWrite) {
				t.Errorf("error = %v, want it to wrap %v", err, errWrite)
			}
		})
	}
}
