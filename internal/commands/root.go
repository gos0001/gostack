package commands

import (
	"github.com/spf13/cobra"
)

// Setting Version is what makes cobra register the --version flag; a separate
// subcommand would be redundant.
var rootCmd = &cobra.Command{
	Use:     "gostack",
	Short:   "Fullstack Go framework CLI",
	Long:    "gostack — create and scaffold fullstack Go projects (NestJS-inspired)",
	Version: Version(),
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// The default template is "gostack version <v>"; install.sh parses this
	// line, so keep it to one field the shell can cut.
	rootCmd.SetVersionTemplate("gostack {{.Version}}\n")

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(devCmd)
}
