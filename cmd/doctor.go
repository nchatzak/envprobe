package cmd

import (
	"fmt"

	"github.com/nchatzak/envprobe/internal/probe"

	"github.com/spf13/cobra"
)

func newDoctorCmd(load func() ([]probe.Check, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check that required developer tools are installed",
		Long:  `Check that required developer tools are installed. This command will check for the presence of required tools and their versions, and report any issues found.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
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
	if jsonFlag {
		probe.RenderJSON(cmd.OutOrStdout(), results)
	} else {
		probe.Render(cmd.OutOrStdout(), results)
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
