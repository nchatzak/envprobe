package cmd

import (
	"errors"
	"fmt"

	"github.com/nchatzak/envprobe/internal/probe"

	"github.com/spf13/cobra"
)

// errNoChecks means the config parsed but defined no checks, under --ci. A
// gate that passes without verifying anything is broken, so this is a failure
// there even though plain doctor treats an empty list as a valid choice.
var errNoChecks = errors.New("no checks configured, so nothing was verified")

// errChecksFailed means the checks ran and some reported a problem.
var errChecksFailed = errors.New("checks failed")

// doctorOptions holds the parsed values of doctor's flags.
type doctorOptions struct {
	json bool
	ci   bool
}

func newDoctorCmd(load checkLoader) *cobra.Command {
	var opts doctorOptions

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run the configured environment checks",
		Long: `Run every check in your envprobe config and report what passed.

Checks are read from envprobe.yaml in the current directory, your home
directory, or ~/.config/envprobe. Run "envprobe config init" to create one.

Exit codes:
  0  every check passed, or there was nothing to check
  1  the checks ran and at least one failed (--ci only)
  2  envprobe could not check: no config file, no checks under --ci, a
     config it could not build, or a bad flag`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return runDoctorCmd(cmd, load, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "output results as JSON")
	cmd.Flags().BoolVar(&opts.ci, "ci", false, "exit 1 if any check fails, 2 if none are configured")
	return cmd
}

func runDoctorCmd(cmd *cobra.Command, load checkLoader, opts doctorOptions) error {
	checks, source, err := load()

	// Before the error check, so a config that failed to build still names the
	// file.
	printConfigSource(cmd.ErrOrStderr(), source)

	if err != nil {
		return err
	}

	// An empty checks: list is deliberate config, so interactively it is a
	// warning and exit 0 — but the table renderer emits nothing for zero
	// results, so without this a run that checked nothing looks like a crash.
	// Under --ci it is a failure instead, reported here rather than after the
	// render: there is nothing to run and nothing to show.
	if len(checks) == 0 {
		if opts.ci {
			return errNoChecks
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: no checks configured")
	}

	results := probe.RunAll(cmd.Context(), checks)

	render := probe.Render
	if opts.json {
		render = probe.RenderJSON
	}

	if err := render(cmd.OutOrStdout(), results); err != nil {
		return fmt.Errorf("rendering results: %w", err)
	}

	probe.PrintSummary(cmd.ErrOrStderr(), results)

	// After render on purpose: --ci --json still emits its results before the
	// non-zero exit.
	if opts.ci && probe.CountFailed(results) > 0 {
		return errChecksFailed
	}
	return nil
}
