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

type fakeCheck struct {
	result probe.Result
}

func (f fakeCheck) Run(ctx context.Context) probe.Result {
	return f.result
}

func runDoctor(t *testing.T, load func() ([]probe.Check, error), args ...string) (stdout, stderr string, err error) {
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

func staticLoader(checks ...probe.Check) func() ([]probe.Check, error) {
	return func() ([]probe.Check, error) {
		return checks, nil
	}
}

func failingLoader(err error) func() ([]probe.Check, error) {
	return func() ([]probe.Check, error) {
		return nil, err
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
		wantErr string // "" means: expect success
	}{
		{"all pass", []probe.Check{found("git"), found("docker")}, ""},
		{"one fails", []probe.Check{found("git"), missing("docker")}, "1 of 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, _, err := runDoctor(t, staticLoader(tt.checks...), "--ci")
			// A failing check is a runtime failure, not misuse: no usage block.
			if strings.Contains(stdout, "Usage:") {
				t.Errorf("doctor printed usage for a runtime failure:\n%s", stdout)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestDoctorLoaderError(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	out, _, err := runDoctor(t, failingLoader(errBoom))
	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
	if out != "" {
		t.Errorf("expected no output, got:\n%s", out)
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
