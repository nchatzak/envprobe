package cmd

import (
	"fmt"

	"github.com/nchatzak/envprobe/internal/probe"

	"github.com/spf13/cobra"
)

func newDoctorCmd(load func() ([]probe.Check, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run the configured environment checks",
		Long: `Run every check in your envprobe config and report what passed.

Checks are read from envprobe.yaml in the current directory, your home
directory, or ~/.config/envprobe. With no config file, a built-in default
set is used — run "envprobe config init" to start from an example.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return runDoctorCmd(cmd, load)
		},
	}
	cmd.Flags().Bool("json", false, "output results as JSON")
	cmd.Flags().Bool("ci", false, "exit non-zero if any check fails")
	return cmd
}

func runDoctorCmd(cmd *cobra.Command, load func() ([]probe.Check, error)) error {
	checks, err := load()
	if err != nil {
		return err
	}

	results := probe.RunAll(cmd.Context(), checks)

	jsonFlag, _ := cmd.Flags().GetBool("json")
	render := probe.Render
	if jsonFlag {
		render = probe.RenderJSON
	}

	if err := render(cmd.OutOrStdout(), results); err != nil {
		return fmt.Errorf("rendering results: %w", err)
	}

	ci, _ := cmd.Flags().GetBool("ci")
	if ci {
		failedCount := probe.CountFailed(results)
		if failedCount > 0 {
			return fmt.Errorf("%d of %d checks failed", failedCount, len(results))
		}
	}
	return nil
}
