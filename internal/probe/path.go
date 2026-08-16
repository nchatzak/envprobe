package probe

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	pathKindFile = "file"
	pathKindDir  = "dir"
)

type pathCheck struct {
	name   string
	target string // ~ already expanded
	kind   string // "", "file" or "dir"
}

var _ Check = pathCheck{}

func (p pathCheck) Run(_ context.Context) Result {
	info, err := os.Stat(p.target)
	if err != nil {
		return Result{Name: p.name, Found: false, Path: p.target, Problem: statProblem(err)}
	}

	if p.kind == pathKindDir && !info.IsDir() {
		return Result{Name: p.name, Found: false, Path: p.target, Problem: problemNotADirectory}
	}

	if p.kind == pathKindFile && !info.Mode().IsRegular() {
		return Result{Name: p.name, Found: false, Path: p.target, Problem: problemNotAFile}
	}

	return Result{Name: p.name, Found: true, Path: p.target}
}

// statProblem names the cause a stat failed. A missing path names none.
func statProblem(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ""
	case errors.Is(err, fs.ErrPermission):
		return problemPermissionDenied
	default:
		return problemCannotRead
	}
}

type pathConfig struct {
	Target string
	Kind   string
}

func newPathCheck(name string, with map[string]any) (Check, error) {
	var cfg pathConfig
	decodeErr := decodeWith(with, &cfg)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode path config: %w", decodeErr)
	}

	if cfg.Target == "" {
		return nil, ErrTargetRequired
	}

	if cfg.Kind != "" && cfg.Kind != pathKindDir && cfg.Kind != pathKindFile {
		return nil, fmt.Errorf("%w %q (one of: %s, %s)", ErrUnknownPathKind, cfg.Kind, pathKindDir, pathKindFile)
	}

	if cfg.Target == "~" || strings.HasPrefix(cfg.Target, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get user home dir: %w", err)
		}
		cfg.Target = filepath.Join(home, strings.TrimPrefix(cfg.Target, "~"))
	}

	return pathCheck{name: name, target: cfg.Target, kind: cfg.Kind}, nil
}
