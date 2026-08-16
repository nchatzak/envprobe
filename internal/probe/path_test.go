package probe

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathCheckRun(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file")
	err := os.WriteFile(filePath, nil, 0o644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	symlinkPath := filepath.Join(dir, "symlink")
	err = os.Symlink(filePath, symlinkPath)
	if err != nil {
		t.Fatalf("failed to symlink file: %v", err)
	}
	nonExistentPath := filepath.Join(dir, "noo")

	tests := []struct {
		name        string
		target      string
		kind        string
		wantFound   bool
		wantProblem string
	}{
		{name: "file found with no kind", target: filePath, kind: "", wantFound: true, wantProblem: ""},
		{name: "dir found with no kind", target: dir, kind: "", wantFound: true, wantProblem: ""},
		{name: "not existing", target: nonExistentPath, kind: "", wantFound: false, wantProblem: ""},
		// A file used as a directory component: ENOTDIR, which is not
		// fs.ErrNotExist and must not render as a plain absence.
		{name: "under a file", target: filepath.Join(filePath, "child"), wantFound: false, wantProblem: problemCannotRead},
		{name: "file found", target: filePath, kind: "file", wantFound: true, wantProblem: ""},
		{name: "not a directory", target: filePath, kind: "dir", wantFound: false, wantProblem: problemNotADirectory},
		{name: "not a file", target: dir, kind: "file", wantFound: false, wantProblem: problemNotAFile},
		{name: "dir found", target: dir, kind: "dir", wantFound: true, wantProblem: ""},
		{name: "symlink", target: symlinkPath, kind: "file", wantFound: true, wantProblem: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const checkName = "testName"
			check := pathCheck{name: checkName, target: tt.target, kind: tt.kind}

			got := check.Run(t.Context())

			if got.Name != checkName {
				t.Errorf("Name = %v, want %v", got.Name, checkName)
			}
			if got.Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", got.Found, tt.wantFound)
			}
			if got.Problem != tt.wantProblem {
				t.Errorf("Problem = %v, want %v", got.Problem, tt.wantProblem)
			}
			if got.Path != tt.target {
				t.Errorf("Path = %v, want %v", got.Path, tt.target)
			}
		})
	}
}

func TestPathCheckPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file modes, so the stat would succeed")
	}

	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	target := filepath.Join(locked, "secret")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// 0o000 makes the directory unsearchable, so the stat below fails at the
	// lookup and never learns whether target exists.
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("failed to chmod dir: %v", err)
	}
	// Registered after t.TempDir's, so it runs first and lets RemoveAll in.
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	const checkName = "testName"
	check := pathCheck{name: checkName, target: target}

	got := check.Run(t.Context())

	if got.Name != checkName {
		t.Errorf("Name = %v, want %v", got.Name, checkName)
	}
	if got.Found {
		t.Error("Found = true, want false")
	}
	if got.Problem != problemPermissionDenied {
		t.Errorf("Problem = %v, want %v", got.Problem, problemPermissionDenied)
	}
	if got.Path != target {
		t.Errorf("Path = %v, want %v", got.Path, target)
	}
}

func TestNewPathCheck(t *testing.T) {
	t.Run("target missing", func(t *testing.T) {
		_, err := newPathCheck("ssh config", map[string]any{"kind": "dir"})
		if !errors.Is(err, ErrTargetRequired) {
			t.Errorf("err = %v, want %v", err, ErrTargetRequired)
		}
	})

	t.Run("unknown kind", func(t *testing.T) {
		_, err := newPathCheck("ssh config", map[string]any{"target": "/opt/envprobe-not-a-real-path", "kind": "folder"})
		if !errors.Is(err, ErrUnknownPathKind) {
			t.Errorf("err = %v, want %v", err, ErrUnknownPathKind)
		}
		wantErrContains(t, err, `"folder"`, "one of: ")
	})

	t.Run("unknown key", func(t *testing.T) {
		_, err := newPathCheck("ssh config", map[string]any{"target": "/opt/envprobe-not-a-real-path", "knid": "dir"})
		if err == nil {
			t.Fatal("newPathCheck() error = nil, want an error")
		}
		wantErrContains(t, err, "has invalid keys", "knid")
	})

	const homePath = "/home/test"

	t.Run("~/ is expanding", func(t *testing.T) {
		t.Setenv("HOME", homePath)
		check := mustPathCheck(t, "ssh config", map[string]any{"target": "~/.ssh/config"})

		const wantTarget = "/home/test/.ssh/config"
		if check.target != wantTarget {
			t.Errorf("target = %v, want %v", check.target, wantTarget)
		}
	})

	t.Run("~ is expanding", func(t *testing.T) {
		t.Setenv("HOME", homePath)
		check := mustPathCheck(t, "home", map[string]any{"target": "~"})

		if check.target != homePath {
			t.Errorf("target = %v, want %v", check.target, homePath)
		}
	})

	t.Run("~ not followed by a slash is left alone", func(t *testing.T) {
		const path = "~folder"
		check := mustPathCheck(t, "home", map[string]any{"target": path})

		if check.target != path {
			t.Errorf("target = %v, want %v", check.target, path)
		}
	})

	t.Run("paths not containing home dir pass through", func(t *testing.T) {
		const dir = "/opt/envprobe-not-a-real-path"
		check := mustPathCheck(t, "home", map[string]any{"target": dir, "kind": "dir"})
		if check.target != dir {
			t.Errorf("target = %v, want %v", check.target, dir)
		}
		if check.kind != "dir" {
			t.Errorf("kind = %v, want %v", check.kind, "dir")
		}
	})
}

// wantErrContains asserts the parts of the message the sentinel cannot carry:
// the offending value and the hint that names the alternatives.
func wantErrContains(t *testing.T, err error, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func mustPathCheck(t *testing.T, name string, with map[string]any) pathCheck {
	t.Helper()
	check, err := newPathCheck(name, with)
	if err != nil {
		t.Fatalf("newPathCheck(%q) error = %v", name, err)
	}
	return check.(pathCheck)
}
