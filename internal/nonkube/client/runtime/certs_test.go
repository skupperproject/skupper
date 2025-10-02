package runtime

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestTlsConfigRetriever(t *testing.T) {
	var err error
	NamespacesPath, err = os.MkdirTemp(os.TempDir(), "tls-config-retriever-*")
	assert.Assert(t, err)

	defer func() {
		t.Logf("Cleaning up %s", NamespacesPath)
		//err = os.RemoveAll(NamespacesPath)
		NamespacesPath = ""
	}()

	baseCertsPath := path.Join(NamespacesPath, "default", string(api.CertificatesPath))
	t.Logf("Creating %s", baseCertsPath)
	err = os.MkdirAll(baseCertsPath, 0755)
	assert.Assert(t, err)

	t.Run("tls-config-retriever-no-certs", func(t *testing.T) {
		tlsCert := GetRuntimeTlsCert("default", "invalid-certificate")
		err = tlsCert.LoadTlsConfig()
		assert.Assert(t, errors.Is(err, fs.ErrNotExist))
	})

	t.Run("tls-config-retriever-valid-certs", func(t *testing.T) {
		validity, err := time.ParseDuration("1h")
		assert.Assert(t, err)
		ca, err := certs.GenerateSecret("my-ca", "my-ca", []string{"my-ca.host"}, validity, nil)
		if err != nil {
			t.Error(err)
		}

		certPath := path.Join(baseCertsPath, "valid-certificate")
		assert.Assert(t, os.MkdirAll(certPath, 0755))
		assert.Assert(t, os.WriteFile(path.Join(certPath, "ca.crt"), ca.Data["ca.crt"], 0644))
		assert.Assert(t, os.WriteFile(path.Join(certPath, "tls.crt"), ca.Data["tls.crt"], 0644))
		assert.Assert(t, os.WriteFile(path.Join(certPath, "tls.key"), ca.Data["tls.key"], 0644))

		tlsCert := GetRuntimeTlsCert("default", "valid-certificate")
		assert.Equal(t, path.Join(certPath, "ca.crt"), tlsCert.CaPath)
		assert.Equal(t, path.Join(certPath, "tls.crt"), tlsCert.CertPath)
		assert.Equal(t, path.Join(certPath, "tls.key"), tlsCert.KeyPath)
		tlsCfg, err := tlsCert.GetTlsConfig()
		assert.Assert(t, err)
		assert.Assert(t, tlsCfg != nil)
	})
}
