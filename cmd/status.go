package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Umono status",
	Long:  `Show the running status of Umono application in the current directory.`,
	Run:   runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	umonoPath := filepath.Join(cwd, "umono")
	umonoExists := true
	if _, err := os.Stat(umonoPath); os.IsNotExist(err) {
		umonoExists = false
	}

	if !umonoExists {
		fmt.Println("⚠️  Not an Umono project (no umono executable found)")
		return
	}

	port := readPortFromEnv(cwd)

	pidPath := filepath.Join(cwd, ".PID")
	pidData, err := os.ReadFile(pidPath)

	if os.IsNotExist(err) {
		fmt.Println("⏹️ Umono is stopped")
		if port != "" {
			fmt.Printf("   Port: %s\n", port)
		}
		return
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read .PID file: %v\n", err)
		os.Exit(1)
	}

	pidStr := strings.TrimSpace(string(pidData))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fmt.Println("⚠️  Invalid .PID file")
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("⏹️ Umono is stopped (stale .PID file)")
		if port != "" {
			fmt.Printf("   Port: %s\n", port)
		}
		return
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		fmt.Println("⏹️ Umono is stopped (stale .PID file)")
		if port != "" {
			fmt.Printf("   Port: %s\n", port)
		}
		return
	}

	fmt.Printf("✅ Umono is running (PID: %d)\n", pid)
	if port != "" {
		fmt.Printf("   Port: %s\n", port)
		fmt.Printf("   URL:  http://localhost:%s\n", port)
	}
}

func readPortFromEnv(dir string) string {
	envPath := filepath.Join(dir, ".env")
	file, err := os.Open(envPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "PORT" {
			return value
		}
	}

	return ""
}
