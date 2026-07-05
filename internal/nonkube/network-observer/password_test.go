package networkobserver

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerateHtpasswdCredentials_UsesProvidedPasswordAndHashesStoredValue(t *testing.T) {
	username, password, htpasswdContent, err := GenerateHtpasswdCredentials("foo", "bar")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if username != "foo" {
		t.Fatalf("expected username %q, got %q", "foo", username)
	}
	if password != "bar" {
		t.Fatalf("expected password %q, got %q", "bar", password)
	}
	if strings.Contains(htpasswdContent, "bar") {
		t.Fatalf("expected htpasswd content to avoid plain password, got %q", htpasswdContent)
	}
	// bcrypt hashes start with $2a$ or $2b$
	if !strings.HasPrefix(htpasswdContent, "foo:$2") {
		t.Fatalf("expected bcrypt htpasswd entry, got %q", htpasswdContent)
	}
	if !strings.HasSuffix(htpasswdContent, "\n") {
		t.Fatalf("expected trailing newline, got %q", htpasswdContent)
	}
	// verify the hash actually matches the password
	hash := strings.TrimSuffix(strings.TrimPrefix(htpasswdContent, "foo:"), "\n")
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("bar")); err != nil {
		t.Fatalf("bcrypt hash does not match password: %v", err)
	}
}

func TestGenerateHtpasswdCredentials_GeneratesPasswordAndHashesStoredValue(t *testing.T) {
	username, password, htpasswdContent, err := GenerateHtpasswdCredentials("foo", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if username != "foo" {
		t.Fatalf("expected username %q, got %q", "foo", username)
	}
	if password == "" {
		t.Fatal("expected generated password")
	}
	if strings.Contains(htpasswdContent, password) {
		t.Fatalf("expected htpasswd content to avoid plain password, got %q", htpasswdContent)
	}
	// bcrypt hashes start with $2a$ or $2b$
	if !strings.HasPrefix(htpasswdContent, "foo:$2") {
		t.Fatalf("expected bcrypt htpasswd entry, got %q", htpasswdContent)
	}
	if !strings.HasSuffix(htpasswdContent, "\n") {
		t.Fatalf("expected trailing newline, got %q", htpasswdContent)
	}
	// verify the hash actually matches the generated password
	hash := strings.TrimSuffix(strings.TrimPrefix(htpasswdContent, "foo:"), "\n")
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Fatalf("bcrypt hash does not match generated password: %v", err)
	}
}
