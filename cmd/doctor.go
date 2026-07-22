package cmd

import (
	"github.com/nchatzak/devsetup/internal/doctor"

	"github.com/spf13/cobra"
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that required developer tools are installed",
	Long:  `Check that required developer tools are installed. This command will check for the presence of required tools and their versions, and report any issues found.`,
	Run: func(cmd *cobra.Command, args []string) {
		tools := []string{"git", "java", "go", "mvn", "docker", "node", "npm", "python"}

		results := doctor.CheckAll(tools)
		doctor.Render(cmd.OutOrStdout(), results)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
