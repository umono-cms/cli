package cmd

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	crypto "github.com/umono-cms/crypto"
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

func TestSecretRotateReencryptsDatabaseAndUpdatesEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	oldKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	oldSecret, err := crypto.New(oldKey, []byte(umonoSecretCryptoInfo))
	if err != nil {
		t.Fatalf("crypto.New() error = %v", err)
	}

	envContent := "# comment\nUMONO_SECRET=" + oldKey.String() + "\nAPP_ENV=prod\nDSN=umono.db\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	plaintexts := map[string][]byte{
		"alpha": []byte("first secret"),
		"beta":  []byte("second secret"),
	}
	writeSecretsDB(t, filepath.Join(dir, "umono.db"), oldSecret, plaintexts)

	withWorkingDir(t, dir, func() {
		var output bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&output)

		if err := runSecretRotate(cmd); err != nil {
			t.Fatalf("runSecretRotate() error = %v", err)
		}

		if !strings.Contains(output.String(), "UMONO_SECRET rotated successfully") {
			t.Fatalf("output = %q, want success message", output.String())
		}
	})

	if _, err := os.Stat(filepath.Join(dir, ".env.old")); !os.IsNotExist(err) {
		t.Fatalf(".env.old stat error = %v, want not exist", err)
	}

	gotEnv, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	gotEnvString := string(gotEnv)
	if strings.Contains(gotEnvString, oldKey.String()) {
		t.Fatal(".env still contains old UMONO_SECRET")
	}
	if !strings.Contains(gotEnvString, "# comment\nUMONO_SECRET=") || !strings.Contains(gotEnvString, "\nAPP_ENV=prod\nDSN=umono.db\n") {
		t.Fatalf(".env content = %q, want non-secret content preserved", gotEnvString)
	}

	newSecretHex, ok, err := envFileValue(filepath.Join(dir, ".env"), "UMONO_SECRET")
	if err != nil {
		t.Fatalf("envFileValue() error = %v", err)
	}
	if !ok {
		t.Fatal("UMONO_SECRET missing from .env")
	}
	newKey, err := crypto.ParseHexString(newSecretHex)
	if err != nil {
		t.Fatalf("ParseHexString() error = %v", err)
	}
	newSecret, err := crypto.New(newKey, []byte(umonoSecretCryptoInfo))
	if err != nil {
		t.Fatalf("crypto.New() error = %v", err)
	}

	db := openTestDB(t, filepath.Join(dir, "umono.db"))
	defer db.Close()

	rows, err := db.Query("SELECT id, ciphertext FROM secrets ORDER BY id")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	for rows.Next() {
		var id string
		var ciphertext []byte
		if err := rows.Scan(&id, &ciphertext); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}

		if _, err := oldSecret.Decrypt(ciphertext, []byte(id)); err == nil {
			t.Fatalf("old secret decrypted rotated record %q", id)
		}

		plaintext, err := newSecret.Decrypt(ciphertext, []byte(id))
		if err != nil {
			t.Fatalf("new secret decrypt record %q error = %v", id, err)
		}
		if !bytes.Equal(plaintext, plaintexts[id]) {
			t.Fatalf("plaintext for %q = %q, want %q", id, plaintext, plaintexts[id])
		}
		seen[id] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Rows error = %v", err)
	}
	if len(seen) != len(plaintexts) {
		t.Fatalf("seen records = %d, want %d", len(seen), len(plaintexts))
	}
}

func TestSecretRotateMissingSecretPrintsInitMessage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_ENV=prod\nDSN=umono.db\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	withWorkingDir(t, dir, func() {
		var output bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&output)

		if err := runSecretRotate(cmd); err != nil {
			t.Fatalf("runSecretRotate() error = %v", err)
		}
		if !strings.Contains(output.String(), "Run 'umono secret init' first") {
			t.Fatalf("output = %q, want init message", output.String())
		}
	})

	if _, err := os.Stat(filepath.Join(dir, ".env.old")); !os.IsNotExist(err) {
		t.Fatalf(".env.old stat error = %v, want not exist", err)
	}
}

func TestSecretRotateKeepsBackupOnFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	oldKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	envContent := "UMONO_SECRET=" + oldKey.String() + "\nDSN=umono.db\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	db := openTestDB(t, filepath.Join(dir, "umono.db"))
	if _, err := db.Exec("CREATE TABLE secrets (id TEXT PRIMARY KEY, ciphertext BLOB NOT NULL)"); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if _, err := db.Exec("INSERT INTO secrets (id, ciphertext) VALUES (?, ?)", "broken", []byte("not encrypted")); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	db.Close()

	withWorkingDir(t, dir, func() {
		var output bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&output)

		err := runSecretRotate(cmd)
		if err == nil {
			t.Fatal("runSecretRotate() error = nil, want decrypt failure")
		}
		if !strings.Contains(err.Error(), "failed to decrypt secret") {
			t.Fatalf("error = %v, want decrypt failure", err)
		}
	})

	if _, err := os.Stat(filepath.Join(dir, ".env.old")); err != nil {
		t.Fatalf(".env.old stat error = %v, want backup to remain", err)
	}

	gotEnv, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(gotEnv) != envContent {
		t.Fatalf(".env = %q, want original content", string(gotEnv))
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("Chdir() restore error = %v", err)
		}
	}()

	fn()
}

func writeSecretsDB(t *testing.T, path string, secret *crypto.Secret, plaintexts map[string][]byte) {
	t.Helper()

	db := openTestDB(t, path)
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE secrets (id TEXT PRIMARY KEY, ciphertext BLOB NOT NULL)"); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	for id, plaintext := range plaintexts {
		ciphertext, err := secret.Encrypt(plaintext, []byte(id))
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}
		if _, err := db.Exec("INSERT INTO secrets (id, ciphertext) VALUES (?, ?)", id, ciphertext); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	}
}

func openTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	return db
}
