// cleanhttp has convenience functions for creating clean http clients and
// transports - free of the globally mutable state in http.DefaultClient and
// http.DefaultTransport variables.
package cleanhttp

import (
	"net"
	"net/http"
	"time"
)

// DefaultClient returns a clean http Client using the same defaults as go's
// http.DefaultClient without relying on global state shared with other
// clients.
func DefaultClient() *http.Client {
	return &http.Client{
		Transport: DefaultTransport(),
	}
}

// DefaultTransport returns a clean http Transport with the same defaults as
// go's http.DefaultTransport.
func DefaultTransport() *http.Transport {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return transport
}
