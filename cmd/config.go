package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/nchatzak/envprobe/internal/probe"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and generate envprobe configuration",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Write an example config to a file",
		Args:  cobra.NoArgs,
		RunE:  runConfigInit,
	}
	initCmd.Flags().StringP("out", "o", "envprobe.yaml", "output path")
	initCmd.Flags().Bool("force", false, "overwrite an existing file")

	validateCmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a config file, or the one envprobe would use",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConfigValidate,
	}

	exampleCmd := &cobra.Command{
		Use:   "example",
		Short: "Print an example config to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			_, err := io.WriteString(cmd.OutOrStdout(), probe.ExampleConfig)
			return err
		},
	}

	cmd.AddCommand(initCmd, validateCmd, exampleCmd)
	return cmd
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	output, _ := cmd.Flags().GetString("out")
	force, _ := cmd.Flags().GetBool("force")

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	f, err := os.OpenFile(output, flags, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s already exists (use --force to overwrite)", output)
	}

	if err != nil {
		return err
	}

	if _, err := io.WriteString(f, probe.ExampleConfig); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", output, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", output)
	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	v, err := configViper(args)
	if err != nil {
		return err
	}

	checks, err := checksFromConfig(v)
	if err != nil {
		return err
	}

	// Non-empty: both loaders return an error when they read no file.
	path := v.ConfigFileUsed()
	if len(checks) == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s defines no checks\n", path)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s: %d checks OK\n", path, len(checks))
	return nil
}

func configViper(args []string) (*viper.Viper, error) {
	if len(args) == 1 {
		return loadConfigFile(args[0])
	}
	return loadConfig()
}
