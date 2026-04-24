package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	crypto "github.com/umono-cms/crypto"
)

var secretCmd = &cobra.Command{
	Use:               "secret",
	Short:             "Manage encryption secrets",
	PersistentPreRunE: requireUmonoStopped,
}

var secretInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate and store UMONO_SECRET if not already set",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		if !isUmonoProjectDir(cwd) {
			return fmt.Errorf("current directory is not an Umono project")
		}

		envPath := filepath.Join(cwd, ".env")
		exists, err := envFileHasKey(envPath, "UMONO_SECRET")
		if err != nil {
			return fmt.Errorf("failed to inspect .env: %w", err)
		}

		if exists {
			cmd.Println("UMONO_SECRET is already set in .env. 'umono secret init' is only used for the initial UMONO_SECRET generation.")
			return nil
		}

		key, err := crypto.GenerateKey()
		if err != nil {
			return fmt.Errorf("failed to generate UMONO_SECRET: %w", err)
		}

		if err := prependEnvValue(envPath, "UMONO_SECRET", key.String()); err != nil {
			return fmt.Errorf("failed to write UMONO_SECRET to .env: %w", err)
		}

		cmd.Println("UMONO_SECRET was added to .env.")
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

func isUmonoProjectDir(dir string) bool {
	umonoPath := filepath.Join(dir, "umono")
	info, err := os.Stat(umonoPath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func envFileHasKey(path, key string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
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

		if strings.TrimSpace(parts[0]) == key {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func prependEnvValue(path, key, value string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	entry := key + "=" + value + "\n"
	return os.WriteFile(path, append([]byte(entry), content...), 0o644)
}
