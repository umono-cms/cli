package secret

import (
	"encoding/hex"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	value, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(value) != Size {
		t.Fatalf("len(Generate()) = %d, want %d", len(value), Size)
	}
}

func TestGenerateHex(t *testing.T) {
	t.Parallel()

	value, err := GenerateHex()
	if err != nil {
		t.Fatalf("GenerateHex() error = %v", err)
	}

	if len(value) != Size*2 {
		t.Fatalf("len(GenerateHex()) = %d, want %d", len(value), Size*2)
	}

	if _, err := hex.DecodeString(value); err != nil {
		t.Fatalf("GenerateHex() returned non-hex value: %v", err)
	}
}
