package cmd

import (
	"errors"
	"runtime/debug"
	"strings"

	"github.com/nchatzak/envprobe/internal/probe"

	"github.com/spf13/cobra"
)

var version = "dev"

// resolveVersion prefers the ldflags value, falling back to the module version
// the toolchain stamps into the binary.
func resolveVersion(v string, info *debug.BuildInfo) string {
	if v != "dev" || info == nil {
		return v
	}
	switch info.Main.Version {
	case "", "(devel)":
		return v
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}

func newRootCmd() *cobra.Command {
	info, _ := debug.ReadBuildInfo()
	cmd := &cobra.Command{
		Use:   "envprobe",
		Short: "Check that an environment has the tools and services it needs",
		Long: `envprobe checks that a machine is set up the way you expect —
a laptop, a CI runner, or a test or UAT box.

Every check in your config names a type, one of: ` + probe.KnownTypes() + `.
Run "envprobe doctor" to check them all at once.`,
		Version: resolveVersion(version, info),
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
	if errors.Is(err, errChecksFailed) {
		return 1
	}
	return 2
}
