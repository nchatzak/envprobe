package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "envprobe",
	Short:        "Development environment helper",
	SilenceUsage: true,

	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.SetConfigName("envprobe")               // look for envprobe.<ext>
	viper.SetConfigType("yaml")                   // expect YAML
	viper.AddConfigPath(".")                      // look for config in the current directory
	viper.AddConfigPath("$HOME")                  // then home
	viper.AddConfigPath("$HOME/.config/envprobe") // then user config dir

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			fmt.Fprintln(os.Stderr, "config error:", err)
			os.Exit(1)
		}
		// if no config file found, fall through to defaults
	}
}
