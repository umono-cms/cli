package cmd

import (
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart Umono",
	Long:  `Restart the Umono application in the current directory. Equivalent to running 'down' followed by 'up'.`,
	Run:   runRestart,
}

func init() {
	restartCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run in background")
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) {
	runDown(cmd, args)
	runUp(cmd, args)
}
