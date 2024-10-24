package common

import (
	"bytes"
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFileSystemConfigurationRenderer_Render(t *testing.T) {
	testFileSystemConfigurationRendererRender(t, false)
}

func TestFileSystemConfigurationRendererWithInputCertificates_Render(t *testing.T) {
	testFileSystemConfigurationRendererRender(t, true)
}

func testFileSystemConfigurationRendererRender(t *testing.T, addInputCertificates bool) {
	ss := fakeSiteState()
	ss.CreateLinkAccessesCertificates()
	ss.CreateBridgeCertificates()
	customOutputPath, err := os.MkdirTemp("", "fs-config-renderer-*")
	assert.Assert(t, err)
	defer func() {
		//err := os.RemoveAll(customOutputPath)
		//assert.Assert(t, err)
	}()
	if addInputCertificates {
		t.Logf(customOutputPath)
		createInputCertificates(t, customOutputPath)
	}
	fsConfigRenderer := new(FileSystemConfigurationRenderer)
	fsConfigRenderer.customOutputPath = customOutputPath
	assert.Assert(t, fsConfigRenderer.Render(ss))
	customOutputPath = fsConfigRenderer.GetOutputPath(ss)
	for _, dirName := range []string{"certificates", "config", "sources", "runtime"} {
		file, err := os.Stat(path.Join(customOutputPath, dirName))
		assert.Assert(t, err)
		assert.Assert(t, file.IsDir())
	}
	if envPlatform := os.Getenv(types.ENV_PLATFORM); envPlatform != "" {
		t.Skipf("The %s environment variable is set to: %s", types.ENV_PLATFORM, envPlatform)
	}
	expectedFiles := []string{
		"config/router/skrouterd.json",
		"certificates/ca/skupper-site-ca/tls.crt",
		"certificates/ca/skupper-site-ca/tls.key",
		"certificates/ca/skupper-service-ca/tls.crt",
		"certificates/ca/skupper-service-ca/tls.key",
		"certificates/client/client-link-access-one/tls.crt",
		"certificates/client/client-link-access-one/tls.key",
		"certificates/client/client-link-access-one/ca.crt",
		"certificates/server/link-access-one/tls.crt",
		"certificates/server/link-access-one/tls.key",
		"certificates/server/link-access-one/ca.crt",
		"certificates/server/listener-one-credentials/tls.crt",
		"certificates/server/listener-one-credentials/tls.key",
		"certificates/server/listener-one-credentials/ca.crt",
		"certificates/server/listener-two-credentials/tls.crt",
		"certificates/server/listener-two-credentials/tls.key",
		"certificates/server/listener-two-credentials/ca.crt",
		"certificates/server/connector-one-credentials/tls.key",
		"certificates/server/connector-one-credentials/ca.crt",
		"certificates/server/connector-one-credentials/tls.crt",
		"certificates/link/link-one-profile/ca.crt",
		"certificates/link/link-one-profile/tls.crt",
		"certificates/link/link-one-profile/tls.key",
		"runtime/state/platform.yaml",
		"runtime/link/link-link-access-one-127.0.0.1.yaml",
	}
	if !addInputCertificates {
		expectedFiles = append(expectedFiles, "runtime/link/link-link-access-one-localhost.yaml")
	} else {
		expectedFiles = append(expectedFiles, "runtime/link/link-link-access-one-10.0.0.1.yaml")
		expectedFiles = append(expectedFiles, "runtime/link/link-link-access-one-10.0.0.2.yaml")
		expectedFiles = append(expectedFiles, "runtime/link/link-link-access-one-fake.domain.yaml")
	}
	for _, fileName := range expectedFiles {
		fs, err := os.Stat(path.Join(customOutputPath, fileName))
		assert.Assert(t, err)
		assert.Assert(t, fs.Mode().IsRegular())
		assert.Assert(t, fs.Size() > 0)
	}
	if addInputCertificates {
		compareCertificates(t, customOutputPath)
	}
}

func compareCertificates(t *testing.T, customOutputPath string) {
	caPath := path.Join(customOutputPath, "certificates/ca/skupper-site-ca")
	serverPath := path.Join(customOutputPath, "certificates/server/link-access-one")
	clientPath := path.Join(customOutputPath, "certificates/client/client-link-access-one")
	inputCaPath := path.Join(customOutputPath, "input/certificates/ca/skupper-site-ca")
	inputServerPath := path.Join(customOutputPath, "input/certificates/server/link-access-one")
	inputClientPath := path.Join(customOutputPath, "input/certificates/client/client-link-access-one")
	pathsToCompare := map[string]string{
		caPath:     inputCaPath,
		serverPath: inputServerPath,
		clientPath: inputClientPath,
	}
	for certPath, inputCertPath := range pathsToCompare {
		entries, err := os.ReadDir(certPath)
		assert.Assert(t, err)
		assert.Assert(t, len(entries) == 3)
		for _, filename := range []string{"ca.crt", "tls.key", "tls.crt"} {
			activeData, err := os.ReadFile(path.Join(certPath, filename))
			assert.Assert(t, err)
			inputData, err := os.ReadFile(path.Join(inputCertPath, filename))
			assert.Assert(t, err)
			assert.Assert(t, bytes.Equal(activeData, inputData))
		}
	}
}

func createInputCertificates(t *testing.T, customOutputPath string) {
	// preparing certificates
	fakeHosts := "10.0.0.1,10.0.0.2,fake.domain"
	ca := certs.GenerateCASecret("fake-ca", "fake-ca")
	server := certs.GenerateSecret("fake-server-cert", "fake-server-cert", fakeHosts, &ca)
	client := certs.GenerateSecret("fake-client-cert", "fake-client-cert", "", &ca)

	// paths for each provided certificate
	caPath := path.Join(customOutputPath, "namespaces/default/input/certificates/ca/skupper-site-ca")
	serverPath := path.Join(customOutputPath, "namespaces/default/input/certificates/server/link-access-one")
	clientPath := path.Join(customOutputPath, "namespaces/default/input/certificates/client/client-link-access-one")
	certsMap := map[string]corev1.Secret{
		caPath:     ca,
		serverPath: server,
		clientPath: client,
	}

	// writing certificates to disk
	for certPath, secret := range certsMap {
		assert.Assert(t, os.MkdirAll(certPath, 0755))
		for filename, data := range secret.Data {
			assert.Assert(t, os.WriteFile(path.Join(certPath, filename), data, 0644))
		}
	}
}

func fakeSiteState() *api.SiteState {
	return &api.SiteState{
		SiteId: "site-id",
		Site: &v2alpha1.Site{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Site",
				APIVersion: "skupper.io/v2alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "site-name",
			},
			Spec: v2alpha1.SiteSpec{},
		},
		Listeners: map[string]*v2alpha1.Listener{
			"listener-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Listener",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "listener-one",
				},
				Spec: v2alpha1.ListenerSpec{
					RoutingKey:     "listener-one-key",
					Host:           "10.0.0.1",
					Port:           1234,
					TlsCredentials: "listener-one-credentials",
					Type:           "tcp",
				},
			},
			"listener-two": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Listener",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "listener-two",
				},
				Spec: v2alpha1.ListenerSpec{
					RoutingKey:     "listener-two-key",
					Host:           "10.0.0.2",
					Port:           1234,
					TlsCredentials: "listener-two-credentials",
					Type:           "tcp",
				},
			},
		},
		Connectors: map[string]*v2alpha1.Connector{
			"connector-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Connector",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "connector-one",
				},
				Spec: v2alpha1.ConnectorSpec{
					RoutingKey:     "connector-one-key",
					Host:           "connector-one-host",
					Port:           1234,
					TlsCredentials: "connector-one-credentials",
					Type:           "tcp",
				},
			},
		},
		RouterAccesses: map[string]*v2alpha1.RouterAccess{
			"link-access-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "RouterAccess",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "link-access-one",
				},
				Spec: v2alpha1.RouterAccessSpec{
					Roles: []v2alpha1.RouterAccessRole{
						{
							Name: "inter-router",
							Port: 55671,
						},
						{
							Name: "edge",
							Port: 45671,
						},
					},
					TlsCredentials: "link-access-one",
					BindHost:       "127.0.0.1",
					SubjectAlternativeNames: []string{
						"localhost",
					},
				},
			},
		},
		Grants: make(map[string]*v2alpha1.AccessGrant),
		Links: map[string]*v2alpha1.Link{
			"link-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Link",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "link-one",
				},
				Spec: v2alpha1.LinkSpec{
					Endpoints: []v2alpha1.Endpoint{
						{
							Name: "inter-router",
							Host: "127.0.0.1",
							Port: "55671",
						},
						{
							Name: "edge",
							Host: "127.0.0.1",
							Port: "45671",
						},
					},
					TlsCredentials: "link-one",
					Cost:           1,
				},
			},
		},
		Secrets: map[string]*corev1.Secret{
			"link-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "link-one",
				},
				Data: map[string][]byte{
					"ca.crt":  []byte("ca.crt"),
					"tls.crt": []byte("tls.crt"),
					"tls.key": []byte("tls.key"),
				},
			},
		},
		Claims:          make(map[string]*v2alpha1.AccessToken),
		Certificates:    make(map[string]*v2alpha1.Certificate),
		SecuredAccesses: make(map[string]*v2alpha1.SecuredAccess),
	}
}
