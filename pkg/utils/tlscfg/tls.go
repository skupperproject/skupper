package tlscfg

import "crypto/tls"

var (
	tlsCiphers []uint16
)

func init() {
	tlsCiphers = make([]uint16, len(tls.CipherSuites()))
	for i, suite := range tls.CipherSuites() {
		tlsCiphers[i] = suite.ID
	}
}

// Modern TLS Configuration for when TLSv1.3 can be assumed (e.g. when only
// internal clients are expected.)
func Modern() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
	}
}

// Default TLS Configuration excludes cipher suites implemented in crypto/tls
// that have not been marked insecure.
func Default() *tls.Config {
	suites := make([]uint16, len(tlsCiphers))
	copy(suites, tlsCiphers)
	return &tls.Config{
		CipherSuites: suites,
	}
}
