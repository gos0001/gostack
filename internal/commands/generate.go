package commands

import "github.com/spf13/cobra"

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"g"},
	Short:   "Generate project components",
}

func init() {
	generateCmd.AddCommand(generateUCCmd)
	generateCmd.AddCommand(generatePageCmd)
	generateCmd.AddCommand(generateAPICmd)
	generateCmd.AddCommand(generateCRUDCmd)
}
