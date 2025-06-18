package grants

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_setCertificateFromSecret(t *testing.T) {
	simpleSecret, err := tf.secret("simple", "", "My Subject", []string{"foo.com", "bar.org"})
	if err != nil {
		t.Error(err)
	}
	var tests = []struct {
		name             string
		secret           *corev1.Secret
		expectedError    string
		expectedSubject  string
		expectedDNSNames []string
	}{
		{
			name:             "simple",
			secret:           simpleSecret,
			expectedSubject:  "My Subject",
			expectedDNSNames: []string{"foo.com", "bar.org"},
		},
		{
			name:          "non tls secret",
			secret:        tf.genericSecret("simple", ""),
			expectedError: "failed to find any PEM data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newServer(":0", true, &TestHandler{})
			err := server.setCertificateFromSecret(tt.secret)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {
				cert, _ := server.getCertificate(nil)
				assert.Assert(t, cert != nil)
				cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
				if err != nil {
					t.Error(err)
				}
				assert.Equal(t, cert.Leaf.Subject.CommonName, tt.expectedSubject)
				if tt.expectedDNSNames != nil {
					assert.DeepEqual(t, cert.Leaf.DNSNames, tt.expectedDNSNames)
				}
			}
		})
	}
}

func Test_handlesErrorOnListen(t *testing.T) {
	server1 := newServer(":0", true, &TestHandler{})
	err := server1.listen()
	if err != nil {
		t.Error(err)
	}
	defer server1.stop()
	server2 := newServer(fmt.Sprintf(":%d", server1.port()), true, &TestHandler{})
	err = server2.listenAndServe()
	assert.ErrorContains(t, err, server2.server.Addr)
}

func Test_handlesServeBeforeListen(t *testing.T) {
	server := newServer(":0", true, &TestHandler{})
	err := server.serve()
	assert.ErrorContains(t, err, "Cannot serve before listen() is called")
}

func Test_handlesPortBeforeListen(t *testing.T) {
	server := newServer(":1234", true, &TestHandler{})
	assert.Equal(t, 0, server.port())
}

type TestHandler struct{}

func (h *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
