package tlscfg

import (
	"crypto/tls"
	"testing"
)

func TestModern(t *testing.T) {
	config := Modern()

	if config == nil {
		t.Error("Modern() returned nil config")
	}

	if config.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion to be TLSv1.3 (%d), got %d", tls.VersionTLS13, config.MinVersion)
	}
}
