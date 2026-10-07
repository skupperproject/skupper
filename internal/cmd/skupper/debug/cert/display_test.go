package cert

import (
	"testing"

	"github.com/skupperproject/skupper/internal/certs"
	"gotest.tools/v3/assert"
)

func TestParseCertificate(t *testing.T) {
	caSecret, err := certs.GenerateSecret("test-ca", "test-ca.example.com", nil, 0, nil)
	assert.NilError(t, err)

	leafSecret, err := certs.GenerateSecret("test-cert", "test.example.com", []string{"test.example.com"}, 86400000000000, caSecret)
	assert.NilError(t, err)

	info, err := ParseCertificate("test-cert", leafSecret.Data["tls.crt"])
	assert.NilError(t, err)

	assert.Equal(t, "test-cert", info.Name)
	assert.Equal(t, "test.example.com", info.Subject)
	assert.Equal(t, "test-ca.example.com", info.Issuer)
	assert.Equal(t, "RSA", info.PublicKeyAlgorithm)
	assert.Equal(t, 2048, info.PublicKeySize)
	assert.Assert(t, len(info.NotBefore) > 0)
	assert.Assert(t, len(info.NotAfter) > 0)
	assert.DeepEqual(t, []string{"test.example.com"}, info.DNSNames)
}

func TestParseCertificateInvalidPEM(t *testing.T) {
	_, err := ParseCertificate("bad", []byte("not a pem"))
	assert.ErrorContains(t, err, "Could not decode PEM block")
}
