package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvFileHasKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# comment\nAPP_ENV=prod\nUMONO_SECRET=abc123\n"

	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	exists, err := envFileHasKey(envPath, "UMONO_SECRET")
	if err != nil {
		t.Fatalf("envFileHasKey() error = %v", err)
	}

	if !exists {
		t.Fatal("envFileHasKey() = false, want true")
	}
}

func TestPrependEnvValuePreservesContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "APP_ENV=prod"

	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := prependEnvValue(envPath, "UMONO_SECRET", "deadbeef"); err != nil {
		t.Fatalf("prependEnvValue() error = %v", err)
	}

	got, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	want := "UMONO_SECRET=deadbeef\nAPP_ENV=prod"
	if string(got) != want {
		t.Fatalf("prependEnvValue() wrote %q, want %q", string(got), want)
	}
}

func TestIsUmonoProjectDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if !isUmonoProjectDir(dir) {
		t.Fatal("isUmonoProjectDir() = false, want true")
	}
}

func TestSecretInitAddsSecretWhenMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_ENV=prod\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	if err := secretInitCmd.RunE(secretInitCmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	firstLine := lines[0]
	if !strings.HasPrefix(firstLine, "UMONO_SECRET=") {
		t.Fatalf("first line = %q, want UMONO_SECRET entry", firstLine)
	}

	secretValue := strings.TrimPrefix(firstLine, "UMONO_SECRET=")
	if len(secretValue) != 64 {
		t.Fatalf("UMONO_SECRET length = %d, want 64", len(secretValue))
	}
}
