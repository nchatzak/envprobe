package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envprobe",
		Short: "Check that an environment has the tools and services it needs",
		Long: `envprobe checks that a machine is set up the way you expect —
a laptop, a CI runner, or a test or UAT box.

It verifies that binaries are on PATH and reports their versions, that
something is listening on the ports you name, and that the Docker daemon
answers. Run "envprobe doctor" to check them all at once.`,
	}
	cmd.AddCommand(newDoctorCmd(configuredChecks), newConfigCmd())
	return cmd
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
