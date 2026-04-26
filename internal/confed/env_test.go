package confed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrependValueWritesKeyFirst(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.example")
	outPath := filepath.Join(dir, ".env")

	if err := os.WriteFile(envPath, []byte("APP_ENV=dev\nUMONO_SECRET=old\nPORT=8999\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	editor := NewEnvEditor()
	if err := editor.Read(envPath); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if err := editor.PrependValue("UMONO_SECRET", "new").Write(outPath); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	gotString := string(got)
	if !strings.HasPrefix(gotString, "UMONO_SECRET=new\n") {
		t.Fatalf("content = %q, want UMONO_SECRET first", gotString)
	}
	if strings.Contains(gotString, "UMONO_SECRET=old") {
		t.Fatalf("content = %q, want old UMONO_SECRET removed", gotString)
	}
}
