package secret

import (
	"crypto/rand"
	"encoding/hex"
)

const Size = 32

func Generate() ([]byte, error) {
	value := make([]byte, Size)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}

	return value, nil
}

func GenerateHex() (string, error) {
	value, err := Generate()
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(value), nil
}
