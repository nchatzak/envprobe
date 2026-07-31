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

func runDoctor(t *testing.T, load func() ([]probe.Check, error), args ...string) (string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	cmd := newDoctorCmd(load)

	// Detached from newRootCmd, so root's SilenceUsage doesn't apply and a
	// RunE error would dump the usage block into stderr.
	cmd.SilenceUsage = true

	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append([]string{}, args...))

	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), err
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
	out, err := runDoctor(t, staticLoader(found("git"), missing("docker")))
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

	out, err := runDoctor(t, staticLoader(found("git"), missing("docker")), "--json")
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
			_, err := runDoctor(t, staticLoader(tt.checks...), "--ci")
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
	out, err := runDoctor(t, failingLoader(errBoom))
	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
	if out != "" {
		t.Errorf("expected no output, got:\n%s", out)
	}
}
