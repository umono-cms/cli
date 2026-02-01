package project

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/confed"
	"github.com/umono-cms/cli/internal/download"
	"golang.org/x/crypto/bcrypt"
)

type Project struct {
	Username string
	Password string
	Path     string
	Port     string
}

func Create(cmd *cobra.Command, project Project) error {
	client := download.NewClient()

	releaseInfo, err := client.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}

	if err := client.DownloadAndExtract(releaseInfo, project.Path); err != nil {
		return err
	}

	hashedUsername, err := hashData(project.Username)
	if err != nil {
		return fmt.Errorf("failed to hash Username: %w", err)
	}

	hashedPassword, err := hashData(project.Password)
	if err != nil {
		return fmt.Errorf("failed to hash Password: %w", err)
	}

	envEditor := confed.NewEnvEditor()
	envEditor.Read(filepath.Join(project.Path, ".env.example"))
	err = envEditor.SetValue("APP_ENV", "prod").
		SetValue("SESSION_DRIVER", "memory").
		AddBlankLine().
		SetValue("PORT", ":"+project.Port).
		SetValue("DSN", "umono.db").
		AddBlankLine().
		SetValue("USERNAME", "").
		SetValue("PASSWORD", "").
		AddBlankLine().
		SetValue("HASHED_USERNAME", base64.StdEncoding.EncodeToString([]byte(hashedUsername))).
		SetValue("HASHED_PASSWORD", base64.StdEncoding.EncodeToString([]byte(hashedPassword))).
		Write(filepath.Join(project.Path, ".env"))
	if err != nil {
		return fmt.Errorf("failed to write .env", err)
	}

	return nil
}

func Upgrade(projectPath string) error {
	binaryPath := findBinaryPath(projectPath)
	if binaryPath == "" {
		return fmt.Errorf("no Umono binary found in %s", projectPath)
	}

	client := download.NewClient()

	releaseInfo, err := client.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}

	fmt.Printf("📦 Latest version: %s\n", releaseInfo.Version)

	tmpDir, err := os.MkdirTemp("", "umono-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := client.DownloadAndExtract(releaseInfo, tmpDir); err != nil {
		return err
	}

	newBinaryPath := findBinaryPath(tmpDir)
	if newBinaryPath == "" {
		return fmt.Errorf("no binary found in downloaded release")
	}

	backupPath := binaryPath + ".backup"
	if err := os.Rename(binaryPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup existing binary: %w", err)
	}

	if err := copyFile(newBinaryPath, binaryPath); err != nil {
		os.Rename(backupPath, binaryPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	os.Remove(backupPath)

	return nil
}

func findBinaryPath(dir string) string {
	candidates := []string{"umono", "umono-darwin-amd64", "umono-darwin-arm64", "umono-linux-amd64", "umono-linux-arm64"}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}

	return ""
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func hashData(data string) (string, error) {
	hashedData, err := bcrypt.GenerateFromPassword([]byte(data), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedData), nil
}
