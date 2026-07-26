package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "envprobe",
		Short:        "Development environment helper",
		SilenceUsage: true,
	}
	cmd.AddCommand(newDoctorCmd(), newConfigCmd())
	return cmd
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
