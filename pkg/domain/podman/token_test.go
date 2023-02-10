//go:build podman
// +build podman

package podman

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/domain"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestPodmanTokenCertHandler(t *testing.T) {
	// creating a basic site
	assert.Assert(t, createBasicSite())
	defer func() {
		assert.Assert(t, teardownBasicSite())
	}()

	// creating a token file
	tokenHandler := &TokenCertHandler{}
	tokenFile, err := os.CreateTemp(os.TempDir(), "token.*.yaml")
	assert.Assert(t, err)
	_ = tokenFile.Close()
	defer os.Remove(tokenFile.Name())

	// using dummy info for the token
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)
	site, err := siteHandler.Get()
	assert.Assert(t, err)

	tokenCertInfo := &domain.TokenCertInfo{
		InterRouterHost: "127.0.0.1",
		InterRouterPort: string(types.InterRouterListenerPort),
		EdgeHost:        "127.0.0.1",
		EdgePort:        string(types.EdgeListenerPort),
	}

	// creating token in temp file
	err = tokenHandler.Create(tokenFile.Name(), "", tokenCertInfo, site, NewPodmanCredentialHandler(cli))
	assert.Assert(t, err)

	// loading secret from file
	yaml, err := ioutil.ReadFile(tokenFile.Name())
	assert.Assert(t, err)

	// reading yaml as secret
	var secret corev1.Secret
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	_, _, err = serializer.Decode(yaml, nil, &secret)

	// verifying token
	assert.Assert(t, domain.VerifyToken(&secret))
}
