package cmd

import (
	"fmt"

	"github.com/nchatzak/envprobe/internal/probe"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that required developer tools are installed",
	Long:  `Check that required developer tools are installed. This command will check for the presence of required tools and their versions, and report any issues found.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var raws []probe.RawCheck
		if err := viper.UnmarshalKey("checks", &raws); err != nil {
			return fmt.Errorf("invalid checks config: %w", err)
		}
		checks, err := probe.LoadChecks(raws)
		if err != nil {
			return err
		}

		if len(checks) == 0 {
			checks = probe.DefaultChecks()
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
	},
}

func init() {
	doctorCmd.Flags().Bool("json", false, "output results as JSON")
	doctorCmd.Flags().Bool("ci", false, "exit non-zero if any check fails")
	rootCmd.AddCommand(doctorCmd)
}
