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

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and generate envprobe configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write an example envprobe.yaml",
	Args:  cobra.NoArgs,
	RunE:  runConfigInit,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate envprobe configuration file",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConfigValidate,
}

var configExampleCmd = &cobra.Command{
	Use:   "example",
	Short: "Print an example configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := io.WriteString(cmd.OutOrStdout(), probe.ExampleConfig)
		return err
	},
}

func init() {
	configInitCmd.Flags().StringP("out", "o", "envprobe.yaml", `output path`)
	configInitCmd.Flags().Bool("force", false, "overwrite an existing file")
	configCmd.AddCommand(configInitCmd, configValidateCmd, configExampleCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigInit(cmd *cobra.Command, args []string) error {
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

	defer f.Close()

	if _, err := io.WriteString(f, probe.ExampleConfig); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", output)
	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	v, err := configViper(args)
	if err != nil {
		return err
	}

	checks, err := checksFromConfig(v)
	if err != nil {
		return err
	}

	path := v.ConfigFileUsed()
	if path == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: no config file found; doctor will use built-in defaults")
		return nil
	}

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
