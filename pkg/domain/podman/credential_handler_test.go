//go:build podman
// +build podman

package podman

import (
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestPodmanCredentialHandler(t *testing.T) {
	credHandler := NewPodmanCredentialHandler(cli)
	caName := "test-ca-" + utils.RandomId(5)

	// Cert Authorities
	t.Run("ca-create", func(t *testing.T) {
		caSecret, err := credHandler.NewCertAuthority(types.CertAuthority{Name: caName})
		assert.Assert(t, err)
		assert.Assert(t, caSecret != nil)
		assert.Equal(t, caSecret.Name, caName)
		assert.Assert(t, len(caSecret.Data) == 2)
		assert.Assert(t, caSecret.Data["tls.key"] != nil)
		assert.Assert(t, caSecret.Data["tls.crt"] != nil)
	})
	t.Run("ca-list", func(t *testing.T) {
		caList, err := credHandler.ListCertAuthorities()
		assert.Assert(t, err)
		assert.Equal(t, 1, len(caList))
		assert.Equal(t, caName, caList[0].Name)
	})
	t.Run("ca-delete", func(t *testing.T) {
		assert.Assert(t, credHandler.DeleteCertAuthority(caName))
	})

	// Credentials
	credName := "test-cred-" + utils.RandomId(5)
	credSubj := "test.localhost"
	credHosts := []string{"localhost", credSubj}

	cred := types.Credential{
		CA:      caName,
		Name:    credName,
		Subject: credSubj,
		Hosts:   credHosts,
	}
	_, err := credHandler.NewCertAuthority(types.CertAuthority{Name: caName})
	var credSecret *corev1.Secret

	t.Run("cred-create", func(t *testing.T) {
		// ca creation
		assert.Assert(t, err)
		// creating credential
		credSecret, err = credHandler.NewCredential(cred)
		assert.Assert(t, err)

		// validating
		assert.Assert(t, credSecret != nil)
		assert.Equal(t, credSecret.Name, credName)
	})
	t.Run("cred-get", func(t *testing.T) {
		credSecretGet, err := credHandler.GetSecret(credName)
		assert.Assert(t, err)
		assert.DeepEqual(t, credSecretGet, credSecretGet)
	})
	t.Run("cred-list", func(t *testing.T) {
		credList, err := credHandler.ListCredentials()
		assert.Assert(t, err)
		assert.Equal(t, 1, len(credList))
		assert.Equal(t, cred.Name, credList[0].Name)
		assert.Equal(t, cred.CA, credList[0].CA)
		assert.Equal(t, cred.Subject, credList[0].Subject)
		assert.DeepEqual(t, cred.Hosts, credList[0].Hosts)
	})
	t.Run("cred-delete", func(t *testing.T) {
		assert.Assert(t, credHandler.DeleteCredential(cred.Name))
		assert.Assert(t, credHandler.DeleteCertAuthority(caName))
	})
}
