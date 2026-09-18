package application

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

func newResourceID(prefix string) (string, error) {
	if prefix == "" {
		return "", errors.New("resource id prefix is required")
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(buf), nil
}
