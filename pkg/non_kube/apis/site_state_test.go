package apis

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSiteState_IsInterior(t *testing.T) {
	assert.Assert(t, fakeSiteState().IsInterior())
}

func TestSiteState_CreateBridgeCertificates(t *testing.T) {
	ss := fakeSiteState()
	ss.CreateBridgeCertificates()
	assert.Assert(t, len(ss.Listeners) == 2)
	assert.Assert(t, len(ss.Connectors) == 1)
	assert.Assert(t, len(ss.Certificates) == 4)
	_, hasServiceCA := ss.Certificates["skupper-service-ca"]
	assert.Assert(t, hasServiceCA)
	for _, listener := range ss.Listeners {
		if listener.Spec.TlsCredentials == "" {
			continue
		}
		certName := listener.Spec.TlsCredentials
		_, certFound := ss.Certificates[certName]
		assert.Assert(t, certFound)
	}
	for _, connector := range ss.Connectors {
		if connector.Spec.TlsCredentials == "" {
			continue
		}
		certName := connector.Spec.TlsCredentials
		_, certFound := ss.Certificates[certName]
		assert.Assert(t, certFound)
	}
}

func TestSiteState_CreateLinkAccessesCertificates(t *testing.T) {
	ss := fakeSiteState()
	ss.CreateLinkAccessesCertificates()
	assert.Equal(t, len(ss.RouterAccesses), 2)
	assert.Equal(t, len(ss.Certificates), 3)
	_, hasSiteCA := ss.Certificates["skupper-site-ca"]
	assert.Assert(t, hasSiteCA)
	_, hasLinkAccessCert := ss.Certificates["link-access-one"]
	assert.Assert(t, hasLinkAccessCert)
	_, hasLinkAccessToken := ss.Certificates["client-link-access-one"]
	assert.Assert(t, hasLinkAccessToken)
}

func TestSiteState_HasRouterAccess(t *testing.T) {
	assert.Equal(t, fakeSiteState().HasRouterAccess(), true)
}

func TestSiteState_CreateRouterAccess(t *testing.T) {
	ss := NewSiteState(false)
	name := "skupper-local"
	assert.Assert(t, !ss.HasRouterAccess())
	ss.CreateRouterAccess(name, 5671)
	assert.Assert(t, ss.HasRouterAccess())
	_, routerAccessFound := ss.RouterAccesses[name]
	assert.Assert(t, routerAccessFound)
	assert.Equal(t, len(ss.RouterAccesses), 1)
	assert.Equal(t, ss.RouterAccesses[name].Spec.Roles[0].Name, "normal")
	assert.Equal(t, len(ss.Certificates), 3)
	for _, certName := range []string{"skupper-local-ca", "skupper-local-client", "skupper-local-server"} {
		_, certFound := ss.Certificates[certName]
		assert.Assert(t, certFound)
	}
}

func TestSiteState_ToRouterConfig(t *testing.T) {
	ss := fakeSiteState()
	sslProfileBasePath := "${SSL_PROFILE_BASE_PATH}"
	routerConfig := ss.ToRouterConfig(sslProfileBasePath)
	assert.Equal(t, len(routerConfig.Listeners), 3)
	rolesFound := map[string]bool{}
	for _, listener := range routerConfig.Listeners {
		rolesFound[string(listener.Role)] = true
	}
	assert.Equal(t, len(rolesFound), 3, "expecting normal, inter-router and edge, found: %s", rolesFound)
	assert.Equal(t, len(routerConfig.Connectors), 1)
	assert.Equal(t, len(routerConfig.SslProfiles), 3)
	assert.Assert(t, strings.HasPrefix(routerConfig.SslProfiles["link-access-one"].CaCertFile, sslProfileBasePath))
	assert.Assert(t, strings.HasPrefix(routerConfig.SslProfiles["link-one-profile"].CaCertFile, sslProfileBasePath))
	assert.Assert(t, strings.HasPrefix(routerConfig.SslProfiles["local-access-one"].CaCertFile, sslProfileBasePath))
	assert.Equal(t, len(routerConfig.Bridges.TcpListeners), 2)
	assert.Equal(t, len(routerConfig.Bridges.TcpConnectors), 1)
}

func TestMarshalSiteState(t *testing.T) {
	ss := fakeSiteState()
	ss.CreateLinkAccessesCertificates()
	ss.CreateBridgeCertificates()
	dir, err := os.MkdirTemp("", "test-sitestate-*")
	assert.Assert(t, err)
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	assert.Assert(t, MarshalSiteState(*ss, dir))
	expectedFiles := []string{
		"site/site-name.yaml",
		"listeners/listener-one.yaml",
		"listeners/listener-two.yaml",
		"connectors/connector-one.yaml",
		"routerAccesses/link-access-one.yaml",
		"links/link-one.yaml",
		"certificates/skupper-service-ca.yaml",
		"certificates/listener-one-credentials.yaml",
		"certificates/listener-two-credentials.yaml",
		"certificates/connector-one-credentials.yaml",
		"certificates/skupper-site-ca.yaml",
		"certificates/link-access-one.yaml",
		"certificates/client-link-access-one.yaml",
		"secrets/link-one-profile.yaml",
	}
	for _, expectedFile := range expectedFiles {
		info, err := os.Stat(path.Join(dir, expectedFile))
		assert.Assert(t, err)
		assert.Assert(t, info.Mode().IsRegular())
		assert.Assert(t, info.Size() > 0)
	}
}

func fakeSiteState() *SiteState {
	return &SiteState{
		SiteId: "site-id",
		Site: &v1alpha1.Site{
			ObjectMeta: metav1.ObjectMeta{
				Name: "site-name",
			},
			Spec: v1alpha1.SiteSpec{},
		},
		Listeners: map[string]*v1alpha1.Listener{
			"listener-one": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "listener-one",
				},
				Spec: v1alpha1.ListenerSpec{
					RoutingKey:     "listener-one-key",
					Host:           "listener-one-host",
					Port:           1234,
					TlsCredentials: "listener-one-credentials",
					Type:           "tcp",
				},
			},
			"listener-two": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "listener-two",
				},
				Spec: v1alpha1.ListenerSpec{
					RoutingKey:     "listener-two-key",
					Host:           "listener-two-host",
					Port:           1234,
					TlsCredentials: "listener-two-credentials",
					Type:           "tcp",
				},
			},
		},
		Connectors: map[string]*v1alpha1.Connector{
			"connector-one": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "connector-one",
				},
				Spec: v1alpha1.ConnectorSpec{
					RoutingKey:     "connector-one-key",
					Host:           "connector-one-host",
					Port:           1234,
					TlsCredentials: "connector-one-credentials",
					Type:           "tcp",
				},
			},
		},
		RouterAccesses: map[string]*v1alpha1.RouterAccess{
			"link-access-one": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "link-access-one",
				},
				Spec: v1alpha1.RouterAccessSpec{
					Roles: []v1alpha1.RouterAccessRole{
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
			"local-access-one": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "local-access-one",
				},
				Spec: v1alpha1.RouterAccessSpec{
					Roles: []v1alpha1.RouterAccessRole{
						{
							Name: "normal",
							Port: 5671,
						},
					},
					TlsCredentials: "local-access-one",
					BindHost:       "127.0.0.1",
					SubjectAlternativeNames: []string{
						"localhost",
					},
				},
			},
		},
		Grants: make(map[string]*v1alpha1.AccessGrant),
		Links: map[string]*v1alpha1.Link{
			"link-one": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "link-one",
				},
				Spec: v1alpha1.LinkSpec{
					Endpoints: []v1alpha1.Endpoint{
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
			"link-one-profile": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "link-one-profile",
				},
				Data: map[string][]byte{
					"ca.crt":  []byte("ca.crt"),
					"tls.crt": []byte("tls.crt"),
					"tls.key": []byte("tls.key"),
				},
			},
		},
		Claims:          make(map[string]*v1alpha1.AccessToken),
		Certificates:    make(map[string]*v1alpha1.Certificate),
		SecuredAccesses: make(map[string]*v1alpha1.SecuredAccess),
	}
}
