package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run:   runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Fprintln(cmd.OutOrStdout(), version.Display())
}
