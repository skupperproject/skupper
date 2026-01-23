package tlscfg

import (
	"crypto/tls"
	"testing"
)

func TestInit(t *testing.T) {
	if len(tlsCiphers) == 0 {
		t.Error("Expected tlsCiphers to be initialized in init()")
	}

	expectedLength := len(tls.CipherSuites())
	if len(tlsCiphers) != expectedLength {
		t.Errorf("Expected tlsCiphers to have %d suites, got %d", expectedLength, len(tlsCiphers))
	}
}

func TestModern(t *testing.T) {
	config := Modern()

	if config == nil {
		t.Error("Modern() returned nil config")
	}

	if config.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion to be TLSv1.3 (%d), got %d", tls.VersionTLS13, config.MinVersion)
	}
}