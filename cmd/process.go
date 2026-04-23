package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

func runningUmonoPID(dir string) (int, bool, error) {
	pidPath := filepath.Join(dir, ".PID")

	pidData, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to read .PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(pidData))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, false, nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false, nil
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return 0, false, nil
	}

	return pid, true, nil
}

func requireUmonoStopped(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	pid, running, err := runningUmonoPID(cwd)
	if err != nil {
		return err
	}

	if running {
		return fmt.Errorf("cannot run '%s' while Umono is running (PID: %d). Stop it first with 'umono down'", cmd.CommandPath(), pid)
	}

	return nil
}
