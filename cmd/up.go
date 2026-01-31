package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var detach bool

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start Umono",
	Long:  `Start the Umono application in the current directory. If Umono is already running, it will notify you. Use -d flag to run in background (detached mode).`,
	Run:   runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run in background")
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	umonoPath := filepath.Join(cwd, "umono")
	if _, err := os.Stat(umonoPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: umono executable not found in current directory\n")
		os.Exit(1)
	}

	if info, err := os.Stat(umonoPath); err == nil {
		if info.Mode()&0o111 == 0 {
			fmt.Fprintf(os.Stderr, "Error: umono file exists but is not executable\n")
			os.Exit(1)
		}
	}

	pidPath := filepath.Join(cwd, ".PID")
	if pidData, err := os.ReadFile(pidPath); err == nil {
		pidStr := strings.TrimSpace(string(pidData))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			process, err := os.FindProcess(pid)
			if err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
					fmt.Println("Umono is already running (PID:", pid, ")")
					os.Exit(0)
				}
			}
		}
		os.Remove(pidPath)
	}

	if detach {
		execCmd := exec.Command(umonoPath)
		execCmd.Dir = cwd
		execCmd.Stdout = nil
		execCmd.Stderr = nil
		execCmd.Stdin = nil

		execCmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		if err := execCmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start umono: %v\n", err)
			os.Exit(1)
		}

		pid := execCmd.Process.Pid
		if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write PID file: %v\n", err)
		}

		fmt.Println("Umono started in background (PID:", pid, ")")
	} else {
		execCmd := exec.Command(umonoPath)
		execCmd.Dir = cwd
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin

		if err := execCmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start umono: %v\n", err)
			os.Exit(1)
		}

		pid := execCmd.Process.Pid
		if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write PID file: %v\n", err)
		}

		fmt.Println("Umono started (PID:", pid, ")")

		if err := execCmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "Umono exited with error: %v\n", err)
		}

		os.Remove(pidPath)
	}
}
