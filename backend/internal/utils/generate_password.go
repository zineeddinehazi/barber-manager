package utils

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateTempPassword returns a random URL-safe password for newly created
// barber accounts; the barber changes it via PATCH /auth/password.
func GenerateTempPassword() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
