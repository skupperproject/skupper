package common

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
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
		t.Log(customOutputPath)
		createInputCertificates(t, customOutputPath)
	}
	fsConfigRenderer := new(FileSystemConfigurationRenderer)
	fsConfigRenderer.customOutputPath = customOutputPath
	assert.Assert(t, fsConfigRenderer.Render(ss))
	customOutputPath = fsConfigRenderer.GetOutputPath(ss)
	for _, dirName := range []string{"input", "runtime", "internal"} {
		file, err := os.Stat(path.Join(customOutputPath, dirName))
		assert.Assert(t, err)
		assert.Assert(t, file.IsDir())
	}
	if envPlatform := os.Getenv(types.ENV_PLATFORM); envPlatform != "" {
		t.Skipf("The %s environment variable is set to: %s", types.ENV_PLATFORM, envPlatform)
	}
	expectedFiles := []string{
		string(api.RouterConfigPath) + "/skrouterd.json",
		string(api.IssuersPath) + "/skupper-site-ca/tls.crt",
		string(api.IssuersPath) + "/skupper-site-ca/tls.key",
		string(api.IssuersPath) + "/skupper-service-ca/tls.crt",
		string(api.IssuersPath) + "/skupper-service-ca/tls.key",
		string(api.CertificatesPath) + "/client-link-access-one/tls.crt",
		string(api.CertificatesPath) + "/client-link-access-one/tls.key",
		string(api.CertificatesPath) + "/client-link-access-one/ca.crt",
		string(api.CertificatesPath) + "/link-access-one/tls.crt",
		string(api.CertificatesPath) + "/link-access-one/tls.key",
		string(api.CertificatesPath) + "/link-access-one/ca.crt",
		string(api.CertificatesPath) + "/listener-one-credentials/tls.crt",
		string(api.CertificatesPath) + "/listener-one-credentials/tls.key",
		string(api.CertificatesPath) + "/listener-one-credentials/ca.crt",
		string(api.CertificatesPath) + "/listener-two-credentials/tls.crt",
		string(api.CertificatesPath) + "/listener-two-credentials/tls.key",
		string(api.CertificatesPath) + "/listener-two-credentials/ca.crt",
		string(api.CertificatesPath) + "/connector-one-credentials/tls.key",
		string(api.CertificatesPath) + "/connector-one-credentials/ca.crt",
		string(api.CertificatesPath) + "/connector-one-credentials/tls.crt",
		string(api.CertificatesPath) + "/link-one-profile/ca.crt",
		string(api.CertificatesPath) + "/link-one-profile/tls.crt",
		string(api.CertificatesPath) + "/link-one-profile/tls.key",
		string(api.InternalBasePath) + "/platform.yaml",
		string(api.RuntimeTokenPath) + "/link-link-access-one-127.0.0.1.yaml",
	}
	if !addInputCertificates {
		expectedFiles = append(expectedFiles, string(api.RuntimeTokenPath)+"/link-link-access-one-localhost.yaml")
	} else {
		expectedFiles = append(expectedFiles, string(api.RuntimeTokenPath)+"/link-link-access-one-10.0.0.1.yaml")
		expectedFiles = append(expectedFiles, string(api.RuntimeTokenPath)+"/link-link-access-one-10.0.0.2.yaml")
		expectedFiles = append(expectedFiles, string(api.RuntimeTokenPath)+"/link-link-access-one-fake.domain.yaml")
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
	caPath := path.Join(customOutputPath, string(api.IssuersPath), "skupper-site-ca")
	serverPath := path.Join(customOutputPath, string(api.CertificatesPath), "link-access-one")
	clientPath := path.Join(customOutputPath, string(api.CertificatesPath), "client-link-access-one")
	inputCaPath := path.Join(customOutputPath, string(api.InputIssuersPath), "skupper-site-ca")
	inputServerPath := path.Join(customOutputPath, string(api.InputCertificatesPath), "link-access-one")
	inputClientPath := path.Join(customOutputPath, string(api.InputCertificatesPath), "client-link-access-one")
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
	fakeHosts := []string{"10.0.0.1", "10.0.0.2", "fake.domain"}
	ca, err := certs.GenerateSecret("fake-ca", "fake-ca", nil, 0, nil)
	if err != nil {
		t.Error(err)
	}
	server, err := certs.GenerateSecret("fake-server-cert", "fake-server-cert", fakeHosts, 0, ca)
	if err != nil {
		t.Error(err)
	}
	client, err := certs.GenerateSecret("fake-client-cert", "fake-client-cert", nil, 0, ca)
	if err != nil {
		t.Error(err)
	}

	// paths for each provided certificate
	caPath := path.Join(customOutputPath, "namespaces/default", string(api.InputIssuersPath), "skupper-site-ca")
	serverPath := path.Join(customOutputPath, "namespaces/default", string(api.InputCertificatesPath), "link-access-one")
	clientPath := path.Join(customOutputPath, "namespaces/default", string(api.InputCertificatesPath), "client-link-access-one")
	certsMap := map[string]*corev1.Secret{
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
		MultiKeyListeners: map[string]*v2alpha1.MultiKeyListener{
			"mkl-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "MultiKeyListener",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "mkl-one",
				},
				Spec: v2alpha1.MultiKeyListenerSpec{
					Host: "10.0.0.3",
					Port: 5678,
					Strategy: v2alpha1.MultiKeyListenerStrategy{
						Priority: &v2alpha1.PriorityStrategySpec{
							RoutingKeys: []string{"key-primary", "key-secondary"},
						},
					},
				},
			},
			"mkl-two": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "MultiKeyListener",
					APIVersion: "skupper.io/v2alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "mkl-two",
				},
				Spec: v2alpha1.MultiKeyListenerSpec{
					Host: "10.0.0.3",
					Port: 5679,
					Strategy: v2alpha1.MultiKeyListenerStrategy{
						Weighted: &v2alpha1.WeightedStrategySpec{
							RoutingKeys: map[string]uint{"key-primary": 1, "key-secondary": 2},
						},
					},
				},
			},
		},
		ConfigMaps: make(map[string]*corev1.ConfigMap),
	}
}

func renderSiteState(t *testing.T, ss *api.SiteState) string {
	t.Helper()
	ss.CreateLinkAccessesCertificates()
	ss.CreateBridgeCertificates()
	customOutputPath, err := os.MkdirTemp("", "fs-config-renderer-proxy-*")
	assert.Assert(t, err)
	t.Cleanup(func() { os.RemoveAll(customOutputPath) })
	fsConfigRenderer := new(FileSystemConfigurationRenderer)
	fsConfigRenderer.customOutputPath = customOutputPath
	assert.Assert(t, fsConfigRenderer.Render(ss))
	return fsConfigRenderer.GetOutputPath(ss)
}

func readRouterConfig(t *testing.T, outputPath string) []json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path.Join(outputPath, string(api.RouterConfigPath), "skrouterd.json"))
	assert.Assert(t, err)
	var elements []json.RawMessage
	assert.Assert(t, json.Unmarshal(data, &elements))
	return elements
}

func findProxyProfiles(t *testing.T, elements []json.RawMessage) []map[string]interface{} {
	t.Helper()
	var profiles []map[string]interface{}
	for _, elem := range elements {
		var tuple []json.RawMessage
		if json.Unmarshal(elem, &tuple) != nil || len(tuple) != 2 {
			continue
		}
		var typeName string
		if json.Unmarshal(tuple[0], &typeName) != nil {
			continue
		}
		if typeName == "proxyProfile" {
			var obj map[string]interface{}
			assert.Assert(t, json.Unmarshal(tuple[1], &obj))
			profiles = append(profiles, obj)
		}
	}
	return profiles
}

func findConnectorProxyProfile(t *testing.T, elements []json.RawMessage, connectorName string) string {
	t.Helper()
	for _, elem := range elements {
		var tuple []json.RawMessage
		if json.Unmarshal(elem, &tuple) != nil || len(tuple) != 2 {
			continue
		}
		var typeName string
		if json.Unmarshal(tuple[0], &typeName) != nil {
			continue
		}
		if typeName == "connector" {
			var obj map[string]interface{}
			assert.Assert(t, json.Unmarshal(tuple[1], &obj))
			if obj["name"] == connectorName {
				if pp, ok := obj["proxyProfile"]; ok {
					return pp.(string)
				}
				return ""
			}
		}
	}
	return ""
}

func fakeSiteStateWithProxy(proxySecretName string, authenticated bool) *api.SiteState {
	ss := fakeSiteState()
	ss.Links["link-one"].Spec.Settings = map[string]string{
		"proxy-configuration": proxySecretName,
	}
	proxySecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: proxySecretName,
		},
		Type: "kubernetes.io/basic-auth",
		Data: map[string][]byte{
			"host": []byte("proxy.example.com"),
			"port": []byte("3128"),
		},
	}
	if authenticated {
		proxySecret.Data["username"] = []byte("proxyuser")
		proxySecret.Data["password"] = []byte("proxypass")
	}
	ss.Secrets[proxySecretName] = proxySecret
	return ss
}

func TestRender_AuthenticatedProxy(t *testing.T) {
	ss := fakeSiteStateWithProxy("my-proxy-config", true)
	outputPath := renderSiteState(t, ss)

	passwordFile := path.Join(outputPath, string(api.ProxyProfilesPath), "my-proxy-config", "password.txt")
	fi, err := os.Stat(passwordFile)
	assert.Assert(t, err, "password.txt should exist")
	assert.Equal(t, fi.Mode().Perm(), os.FileMode(0600))

	content, err := os.ReadFile(passwordFile)
	assert.Assert(t, err)
	assert.Equal(t, string(content), "proxypass")

	elements := readRouterConfig(t, outputPath)
	profiles := findProxyProfiles(t, elements)
	assert.Equal(t, len(profiles), 1)
	assert.Equal(t, profiles[0]["name"], "my-proxy-config")
	assert.Equal(t, profiles[0]["host"], "proxy.example.com")
	assert.Equal(t, profiles[0]["port"], "3128")
	assert.Equal(t, profiles[0]["username"], "proxyuser")
	password, ok := profiles[0]["password"].(string)
	assert.Assert(t, ok, "password field should be a string")
	assert.Assert(t, strings.HasPrefix(password, "file:"), "password should start with file: prefix, got: %s", password)
	assert.Assert(t, strings.HasSuffix(password, "/my-proxy-config/password.txt"), "password should end with proxy name/password.txt, got: %s", password)

	connectorProxy := findConnectorProxyProfile(t, elements, "link-one")
	assert.Equal(t, connectorProxy, "my-proxy-config")
}

func TestRender_UnauthenticatedProxy(t *testing.T) {
	ss := fakeSiteStateWithProxy("unauth-proxy", false)
	outputPath := renderSiteState(t, ss)

	passwordFile := path.Join(outputPath, string(api.ProxyProfilesPath), "unauth-proxy", "password.txt")
	_, err := os.Stat(passwordFile)
	assert.Assert(t, os.IsNotExist(err), "password.txt should not exist for unauthenticated proxy")

	elements := readRouterConfig(t, outputPath)
	profiles := findProxyProfiles(t, elements)
	assert.Equal(t, len(profiles), 1)
	assert.Equal(t, profiles[0]["name"], "unauth-proxy")
	assert.Equal(t, profiles[0]["host"], "proxy.example.com")
	assert.Equal(t, profiles[0]["port"], "3128")
	_, hasUsername := profiles[0]["username"]
	assert.Assert(t, !hasUsername, "username should not be present for unauthenticated proxy")
	_, hasPassword := profiles[0]["password"]
	assert.Assert(t, !hasPassword, "password should not be present for unauthenticated proxy")
}

func TestRender_MissingProxySecret(t *testing.T) {
	ss := fakeSiteState()
	ss.Links["link-one"].Spec.Settings = map[string]string{
		"proxy-configuration": "nonexistent-proxy",
	}
	outputPath := renderSiteState(t, ss)

	elements := readRouterConfig(t, outputPath)
	profiles := findProxyProfiles(t, elements)
	assert.Equal(t, len(profiles), 0, "no ProxyProfile should be generated for missing Secret")
}

func TestRender_NoProxyConfigured(t *testing.T) {
	ss := fakeSiteState()
	outputPath := renderSiteState(t, ss)

	proxyDir := path.Join(outputPath, string(api.ProxyProfilesPath))
	_, err := os.Stat(proxyDir)
	if err == nil {
		entries, _ := os.ReadDir(proxyDir)
		assert.Equal(t, len(entries), 0, "no proxy directories should exist")
	}

	elements := readRouterConfig(t, outputPath)
	profiles := findProxyProfiles(t, elements)
	assert.Equal(t, len(profiles), 0, "no ProxyProfile should be generated")
}
