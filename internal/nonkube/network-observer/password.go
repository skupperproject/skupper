package networkobserver

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func GenerateHtpasswdCredentials(username string, password string) (string, string, string, error) {
	if password == "" {
		secretBytes := [16]byte{}
		if _, err := rand.Read(secretBytes[:]); err != nil {
			return "", "", "", fmt.Errorf("error generating random password: %w", err)
		}

		password = base64.RawStdEncoding.EncodeToString(secretBytes[:])
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", fmt.Errorf("error generating htpasswd hash: %w", err)
	}

	htpasswdContent := fmt.Sprintf("%s:%s\n", username, string(hash))

	return username, password, htpasswdContent, nil
}
