package api

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
	for _, test := range []struct {
		name   string
		bundle bool
	}{
		{
			name:   "regular-config",
			bundle: false,
		},
		{
			name:   "bundle-config",
			bundle: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ss := fakeSiteState()
			ss.bundle = test.bundle
			sslProfileBasePath := "${SSL_PROFILE_BASE_PATH}"
			routerConfig := ss.ToRouterConfig(sslProfileBasePath, "podman")
			if test.bundle {
				assert.Assert(t, strings.HasSuffix(routerConfig.Metadata.Id, "-{{.SiteNameSuffix}}"))
				assert.Assert(t, strings.Contains(routerConfig.Metadata.Metadata, `"id":"{{.SiteId}}"`), routerConfig.Metadata.Metadata)
				assert.Assert(t, ss.IsBundle())
			} else {
				assert.Assert(t, !strings.HasSuffix(routerConfig.Metadata.Id, "-{{.SiteNameSuffix}}"))
				assert.Assert(t, strings.Contains(routerConfig.Metadata.Metadata, `"id":"site-id"`), routerConfig.Metadata.Metadata)
			}
			assert.Equal(t, len(routerConfig.Listeners), 3)
			rolesFound := map[string]bool{}
			for _, listener := range routerConfig.Listeners {
				rolesFound[string(listener.Role)] = true
			}
			assert.Equal(t, len(rolesFound), 3, "expecting normal, inter-router and edge, found: %s", rolesFound)
			assert.Equal(t, len(routerConfig.Connectors), 2)
			assert.Equal(t, len(routerConfig.SslProfiles), 7)
			assert.Assert(t, strings.HasPrefix(routerConfig.SslProfiles["link-access-one"].CaCertFile, sslProfileBasePath))
			assert.Assert(t, strings.HasPrefix(routerConfig.SslProfiles["link-one-profile"].CaCertFile, sslProfileBasePath))
			assert.Assert(t, strings.HasPrefix(routerConfig.SslProfiles["skupper-local"].CaCertFile, sslProfileBasePath))
			assert.Equal(t, len(routerConfig.Bridges.TcpListeners), 2)
			assert.Equal(t, len(routerConfig.Bridges.TcpConnectors), 1)
			assert.Assert(t, routerConfig.SiteConfig != nil)
			expectedPlatform := "podman"
			expectedNamespace := "default"
			if test.bundle {
				expectedPlatform = "{{.Platform}}"
				expectedNamespace = "{{.Namespace}}"
			}
			assert.Equal(t, routerConfig.SiteConfig.Platform, expectedPlatform)
			assert.Equal(t, routerConfig.SiteConfig.Namespace, expectedNamespace)

		})
	}
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
		"Site-site-name.yaml",
		"Listener-listener-one.yaml",
		"Listener-listener-two.yaml",
		"Connector-connector-one.yaml",
		"RouterAccess-link-access-one.yaml",
		"Link-link-one.yaml",
		"Certificate-skupper-service-ca.yaml",
		"Certificate-listener-one-credentials.yaml",
		"Certificate-listener-two-credentials.yaml",
		"Certificate-connector-one-credentials.yaml",
		"Certificate-skupper-site-ca.yaml",
		"Certificate-link-access-one.yaml",
		"Certificate-client-link-access-one.yaml",
		"Secret-link-one-profile.yaml",
		"ConfigMap-configmap.yaml",
	}
	for _, expectedFile := range expectedFiles {
		info, err := os.Stat(path.Join(dir, expectedFile))
		assert.Assert(t, err)
		assert.Assert(t, info.Mode().IsRegular())
		assert.Assert(t, info.Size() > 0)
	}
}

func TestUpdateStatus(t *testing.T) {
	networkStatus := fakeNetworkStatusInfo()
	ss := fakeSiteState()
	ss.Links["link-one"].Status.RemoteSiteName = "other-site-name"
	ss.Links["link-one"].Status.RemoteSiteId = "other-site-id"
	ss.UpdateStatus(networkStatus)
	assert.Equal(t, ss.Site.Status.SitesInNetwork, 2)
	assert.Equal(t, ss.Listeners["listener-one"].Status.HasMatchingConnector, true)
	assert.Equal(t, ss.Listeners["listener-two"].Status.HasMatchingConnector, false)
	assert.Equal(t, ss.Connectors["connector-one"].Status.HasMatchingListener, true)
	assert.Equal(t, meta.IsStatusConditionTrue(ss.Links["link-one"].Status.Conditions, v2alpha1.CONDITION_TYPE_OPERATIONAL), true)
	assert.Equal(t, meta.IsStatusConditionTrue(ss.Links["link-broken"].Status.Conditions, v2alpha1.CONDITION_TYPE_OPERATIONAL), false)
}

func fakeNetworkStatusInfo() network.NetworkStatusInfo {
	return network.NetworkStatusInfo{
		Addresses: []network.AddressInfo{
			{
				Name:           "listener-one-key",
				Protocol:       "tcp",
				ListenerCount:  1,
				ConnectorCount: 1,
			},
			{
				Name:           "connector-one-key",
				Protocol:       "tcp",
				ListenerCount:  1,
				ConnectorCount: 1,
			},
		},
		SiteStatus: []network.SiteStatusInfo{
			{
				Site: network.SiteInfo{
					Identity:  "site-id",
					Name:      "site-name",
					Namespace: "default",
					Platform:  "podman",
					Version:   "version",
				},
				RouterStatus: []network.RouterStatusInfo{
					{
						Links: []network.LinkInfo{
							{
								Name:     "link-one",
								LinkCost: 1,
								Status:   "up",
								Role:     "inter-router",
								Peer:     "other-site-link-access-identity-inter-router",
							},
						},
						AccessPoints: []network.RouterAccessInfo{
							{Identity: "link-access-one-identity-inter-router"},
							{Identity: "link-access-one-identity-edge"},
						},
						Listeners: []network.ListenerInfo{
							{
								Name:    "listener-one",
								Address: "listener-one-key",
							},
							{
								Name:    "listener-two",
								Address: "listener-two-key",
							},
						},
						Connectors: []network.ConnectorInfo{
							{
								DestHost: "connector-one-host",
								Address:  "connector-one-key",
							},
						},
					},
				},
			}, {
				Site: network.SiteInfo{
					Identity:  "other-site-id",
					Name:      "other-site-name",
					Namespace: "default",
					Platform:  "linux",
					Version:   "version",
				},
				RouterStatus: []network.RouterStatusInfo{
					{
						AccessPoints: []network.RouterAccessInfo{
							{Identity: "other-site-link-access-identity-inter-router"},
							{Identity: "other-site-link-access-identity-edge"},
						},
						Listeners: []network.ListenerInfo{
							{
								Name:    "listener-one",
								Address: "connector-one-key",
							},
						},
						Connectors: []network.ConnectorInfo{
							{
								DestHost: "connector-one-host",
								Address:  "listener-one-key",
							},
						},
					},
				},
			},
		},
	}
}

func fakeSiteState() *SiteState {
	return &SiteState{
		SiteId: "site-id",
		Site: &v2alpha1.Site{
			ObjectMeta: metav1.ObjectMeta{
				Name: "site-name",
			},
			Spec: v2alpha1.SiteSpec{},
		},
		Listeners: map[string]*v2alpha1.Listener{
			"listener-one": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "listener-one",
				},
				Spec: v2alpha1.ListenerSpec{
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
				Spec: v2alpha1.ListenerSpec{
					RoutingKey:     "listener-two-key",
					Host:           "listener-two-host",
					Port:           1234,
					TlsCredentials: "listener-two-credentials",
					Type:           "tcp",
				},
			},
		},
		Connectors: map[string]*v2alpha1.Connector{
			"connector-one": {
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
			"skupper-local": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "skupper-local",
				},
				Spec: v2alpha1.RouterAccessSpec{
					Roles: []v2alpha1.RouterAccessRole{
						{
							Name: "normal",
							Port: 5671,
						},
					},
					TlsCredentials: "skupper-local",
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
			"link-broken": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "link-broken",
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
					TlsCredentials: "link-broken",
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
		ConfigMaps: map[string]*corev1.ConfigMap{
			"configmap": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "configmap",
				},
				Data: map[string]string{
					"data": "data",
				},
			},
		},
		Claims:          make(map[string]*v2alpha1.AccessToken),
		Certificates:    make(map[string]*v2alpha1.Certificate),
		SecuredAccesses: make(map[string]*v2alpha1.SecuredAccess),
	}
}

func TestSetNamespace(t *testing.T) {
	ss := fakeSiteState()

	for _, test := range []struct {
		description   string
		curNamespace  string
		newNamespace  string
		expectChanged bool
	}{
		{
			description:   "empty-to-default",
			curNamespace:  "",
			newNamespace:  "default",
			expectChanged: false,
		},
		{
			description:   "default-to-my-namespace",
			curNamespace:  "default",
			newNamespace:  "my-namespace",
			expectChanged: true,
		},
		{
			description:   "my-namespace-to-other-namespace",
			curNamespace:  "my-namespace",
			newNamespace:  "other-namespace",
			expectChanged: true,
		},
		{
			description:   "other-namespace-to-other-namespace",
			curNamespace:  "other-namespace",
			newNamespace:  "other-namespace",
			expectChanged: false,
		},
		{
			description:   "other-namespace-to-default",
			curNamespace:  "other-namespace",
			newNamespace:  "default",
			expectChanged: true,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			assertNamespaceOnSiteState(t, ss, test.curNamespace)
			ss.SetNamespace(test.newNamespace)
			if test.expectChanged {
				assertNamespaceOnSiteState(t, ss, test.newNamespace)
			} else {
				assertNamespaceOnSiteState(t, ss, test.curNamespace)
			}
		})
	}
}

func assertNamespaceOnSiteState(t *testing.T, ss *SiteState, namespace string) {
	t.Helper()
	assert.Equal(t, ss.GetNamespace(), getDefaultNs(namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.Listeners, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.Connectors, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.RouterAccesses, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.Grants, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.Links, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.Secrets, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.Claims, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.Certificates, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.SecuredAccesses, namespace))
	assert.Assert(t, assertNamespaceOnMap(ss.ConfigMaps, namespace))
}

func assertNamespaceOnMap[T metav1.Object](objMap map[string]T, namespace string) bool {
	for _, obj := range objMap {
		if getDefaultNs(obj.GetNamespace()) != getDefaultNs(namespace) {
			return false
		}
	}
	return true
}

func getDefaultNs(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}
