package cmd

import "github.com/spf13/cobra"

var secretCmd = &cobra.Command{
	Use:               "secret",
	Short:             "Manage encryption secrets",
	PersistentPreRunE: requireUmonoStopped,
}

var secretInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate and store UMONO_SECRET if not already set",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var secretRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate UMONO_SECRET and re-encrypt all stored secrets",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	secretCmd.AddCommand(secretInitCmd)
	secretCmd.AddCommand(secretRotateCmd)
	rootCmd.AddCommand(secretCmd)
}
