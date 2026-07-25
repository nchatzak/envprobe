package cmd

import (
	"github.com/nchatzak/devsetup/internal/doctor"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that required developer tools are installed",
	Long:  `Check that required developer tools are installed. This command will check for the presence of required tools and their versions, and report any issues found.`,
	Run: func(cmd *cobra.Command, args []string) {
		names := viper.GetStringSlice("checks") // retrieve the list of checks from the config file
		checks := doctor.SelectChecks(doctor.DefaultChecks(), names)
		results := doctor.RunAll(cmd.Context(), checks)

		if cmd.Flags().Changed("json") {
			doctor.RenderJSON(cmd.OutOrStdout(), results)
		} else {
			doctor.Render(cmd.OutOrStdout(), results)
		}
	},
}

func init() {
	doctorCmd.Flags().Bool("json", false, "output results as JSON")
	rootCmd.AddCommand(doctorCmd)
}
