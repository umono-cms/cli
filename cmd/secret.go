package cmd

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	crypto "github.com/umono-cms/crypto"
	_ "modernc.org/sqlite"
)

const umonoSecretCryptoInfo = "umono-secrets"

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
		return runSecretRotate(cmd)
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

func envFileValue(path, key string) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		value, ok := parseEnvValue(scanner.Text(), key)
		if ok {
			return value, true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	return "", false, nil
}

func prependEnvValue(path, key, value string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	entry := key + "=" + value + "\n"
	return os.WriteFile(path, append([]byte(entry), content...), 0o644)
}

func runSecretRotate(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if !isUmonoProjectDir(cwd) {
		return fmt.Errorf("current directory is not an Umono project")
	}

	envPath := filepath.Join(cwd, ".env")
	backupPath := filepath.Join(cwd, ".env.old")

	exists, err := envFileHasKey(envPath, "UMONO_SECRET")
	if err != nil {
		return fmt.Errorf("failed to inspect .env: %w", err)
	}
	if !exists {
		cmd.Println("UMONO_SECRET is not set in .env. Run 'umono secret init' first to generate it.")
		return nil
	}

	if _, err := os.Stat(backupPath); err == nil {
		return fmt.Errorf(".env.old already exists. Remove it after recovering the previous rotation before running 'umono secret rotate' again")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect .env.old: %w", err)
	}

	cmd.Println("Creating .env.old backup...")
	if err := copyFile(envPath, backupPath); err != nil {
		return fmt.Errorf("failed to create .env.old backup: %w", err)
	}

	envUpdated := false
	dbCommitted := false
	defer func() {
		if envUpdated && !dbCommitted {
			_ = copyFile(backupPath, envPath)
		}
	}()

	cmd.Println("Reading current UMONO_SECRET and DSN...")
	oldSecretHex, ok, err := envFileValue(backupPath, "UMONO_SECRET")
	if err != nil {
		return fmt.Errorf("failed to read UMONO_SECRET from .env.old: %w", err)
	}
	if !ok {
		return fmt.Errorf("UMONO_SECRET is not set in .env.old")
	}

	dsn, ok, err := envFileValue(backupPath, "DSN")
	if err != nil {
		return fmt.Errorf("failed to read DSN from .env.old: %w", err)
	}
	if !ok || dsn == "" {
		return fmt.Errorf("DSN is not set in .env.old")
	}

	oldKey, err := crypto.ParseHexString(oldSecretHex)
	if err != nil {
		return fmt.Errorf("failed to parse current UMONO_SECRET: %w", err)
	}
	oldSecret, err := crypto.New(oldKey, []byte(umonoSecretCryptoInfo))
	if err != nil {
		return fmt.Errorf("failed to initialize current UMONO_SECRET: %w", err)
	}

	cmd.Println("Opening database...")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	cmd.Println("Generating new UMONO_SECRET...")
	newKey, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate UMONO_SECRET: %w", err)
	}
	newSecret, err := crypto.New(newKey, []byte(umonoSecretCryptoInfo))
	if err != nil {
		return fmt.Errorf("failed to initialize new UMONO_SECRET: %w", err)
	}
	newSecretHex := newKey.String()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start database transaction: %w", err)
	}
	defer tx.Rollback()

	cmd.Println("Re-encrypting stored secrets...")
	expectedPlaintext, err := rotateStoredSecrets(tx, oldSecret, newSecret)
	if err != nil {
		return err
	}

	cmd.Println("Writing new UMONO_SECRET to .env...")
	if err := replaceEnvValue(envPath, "UMONO_SECRET", newSecretHex); err != nil {
		return fmt.Errorf("failed to write UMONO_SECRET to .env: %w", err)
	}
	envUpdated = true

	cmd.Println("Verifying rotated secrets...")
	if err := verifyRotatedSecrets(tx, envPath, newSecretHex, newSecret, expectedPlaintext); err != nil {
		return fmt.Errorf("rotation verification failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit database transaction: %w", err)
	}
	dbCommitted = true

	if err := verifyRotatedSecrets(db, envPath, newSecretHex, newSecret, expectedPlaintext); err != nil {
		return fmt.Errorf("rotation verification failed after commit: %w", err)
	}

	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("rotation completed, but failed to remove .env.old: %w", err)
	}

	cmd.Println("UMONO_SECRET rotated successfully. .env.old was removed.")
	return nil
}

func parseEnvValue(line, key string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", false
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", false
	}

	if strings.TrimSpace(parts[0]) != key {
		return "", false
	}

	return trimEnvQuotes(strings.TrimSpace(parts[1])), true
}

func trimEnvQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	first := value[0]
	last := value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1]
	}

	return value
}

func replaceEnvValue(path, key, value string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := bytes.SplitAfter(content, []byte("\n"))
	replaced := false
	for i, line := range lines {
		lineText := string(line)
		newline := ""
		if strings.HasSuffix(lineText, "\n") {
			newline = "\n"
			lineText = strings.TrimSuffix(lineText, "\n")
			if strings.HasSuffix(lineText, "\r") {
				newline = "\r\n"
				lineText = strings.TrimSuffix(lineText, "\r")
			}
		}

		trimmed := strings.TrimSpace(lineText)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		eq := strings.Index(lineText, "=")
		if eq < 0 || strings.TrimSpace(lineText[:eq]) != key {
			continue
		}

		rhs := lineText[eq+1:]
		leadingSpace := rhs[:len(rhs)-len(strings.TrimLeft(rhs, " \t"))]
		quote := ""
		trimmedRHS := strings.TrimSpace(rhs)
		if len(trimmedRHS) >= 2 {
			first := trimmedRHS[0]
			last := trimmedRHS[len(trimmedRHS)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				quote = string(first)
			}
		}

		lines[i] = []byte(lineText[:eq+1] + leadingSpace + quote + value + quote + newline)
		replaced = true
		break
	}

	if !replaced {
		return fmt.Errorf("%s is not set in .env", key)
	}

	return os.WriteFile(path, bytes.Join(lines, nil), 0o644)
}

type secretQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type storedSecretRecord struct {
	id         string
	ciphertext []byte
}

func rotateStoredSecrets(tx *sql.Tx, oldSecret, newSecret *crypto.Secret) (map[string][]byte, error) {
	rows, err := tx.Query("SELECT id, ciphertext FROM secrets ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets table: %w", err)
	}

	var records []storedSecretRecord
	for rows.Next() {
		var record storedSecretRecord
		if err := rows.Scan(&record.id, &record.ciphertext); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan secret record: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("failed to iterate secrets table: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("failed to close secrets rows: %w", err)
	}

	expectedPlaintext := make(map[string][]byte)
	for _, record := range records {
		recordID := []byte(record.id)
		plaintext, err := oldSecret.Decrypt(record.ciphertext, recordID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret %q: %w", record.id, err)
		}

		nextCiphertext, err := newSecret.Encrypt(plaintext, recordID)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret %q: %w", record.id, err)
		}

		result, err := tx.Exec("UPDATE secrets SET ciphertext = ? WHERE id = ?", nextCiphertext, record.id)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret %q: %w", record.id, err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("failed to verify update for secret %q: %w", record.id, err)
		}
		if affected != 1 {
			return nil, fmt.Errorf("failed to update secret %q: affected rows = %d", record.id, affected)
		}

		expectedPlaintext[record.id] = plaintext
	}

	return expectedPlaintext, nil
}

func verifyRotatedSecrets(queryer secretQueryer, envPath, newSecretHex string, newSecret *crypto.Secret, expectedPlaintext map[string][]byte) error {
	envSecretHex, ok, err := envFileValue(envPath, "UMONO_SECRET")
	if err != nil {
		return fmt.Errorf("failed to read UMONO_SECRET from .env: %w", err)
	}
	if !ok {
		return fmt.Errorf("UMONO_SECRET is missing from .env")
	}
	if envSecretHex != newSecretHex {
		return fmt.Errorf("UMONO_SECRET in .env does not match the generated secret")
	}

	rows, err := queryer.Query("SELECT id, ciphertext FROM secrets ORDER BY id")
	if err != nil {
		return fmt.Errorf("failed to read secrets table: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool, len(expectedPlaintext))
	for rows.Next() {
		var id string
		var ciphertext []byte
		if err := rows.Scan(&id, &ciphertext); err != nil {
			return fmt.Errorf("failed to scan secret record: %w", err)
		}

		expected, ok := expectedPlaintext[id]
		if !ok {
			return fmt.Errorf("unexpected secret record %q after rotation", id)
		}

		plaintext, err := newSecret.Decrypt(ciphertext, []byte(id))
		if err != nil {
			return fmt.Errorf("failed to decrypt rotated secret %q: %w", id, err)
		}
		if !bytes.Equal(plaintext, expected) {
			return fmt.Errorf("rotated secret %q plaintext does not match original value", id)
		}

		seen[id] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate secrets table: %w", err)
	}

	if len(seen) != len(expectedPlaintext) {
		return fmt.Errorf("rotated secrets count = %d, want %d", len(seen), len(expectedPlaintext))
	}

	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	sourceInfo, err := source.Stat()
	if err != nil {
		return err
	}

	destination, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return err
	}

	return destination.Close()
}
