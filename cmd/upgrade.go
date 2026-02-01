package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/project"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Umono to the latest version",
	Long: `Upgrade the current Umono installation to the latest release.

This command will:
  - Check for the latest Umono release
  - Download the new binary for your platform
  - Replace the existing binary while preserving your data

Your database (umono.db) and configuration (.env) will be preserved.

Example:
  cd my-project
  umono upgrade`,
	Run: runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🔄 Checking for updates...")

	err = project.Upgrade(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// TODO: Add feature that up to date.

	fmt.Println("✅ Upgrade completed successfully!")
}
