package tlscfg

import "crypto/tls"

// Modern TLS Configuration for when TLSv1.3 can be assumed (e.g. when only
// internal clients are expected.)
func Modern() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
	}
}
