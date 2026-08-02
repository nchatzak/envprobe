package cmd

import (
	"errors"

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

func Execute() int {
	return exitCode(newRootCmd().Execute())
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if _, ok := errors.AsType[checksFailedError](err); ok {
		return 1
	}
	return 2
}
