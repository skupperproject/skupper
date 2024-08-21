package common

import (
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFileSystemConfigurationRenderer_Render(t *testing.T) {
	ss := fakeSiteState()
	ss.CreateLinkAccessesCertificates()
	ss.CreateBridgeCertificates()
	customOutputPath, err := os.MkdirTemp("", "fs-config-renderer-*")
	assert.Assert(t, err)
	defer func() {
		err := os.RemoveAll(customOutputPath)
		assert.Assert(t, err)
	}()
	fsConfigRenderer := new(FileSystemConfigurationRenderer)
	fsConfigRenderer.customOutputPath = customOutputPath
	assert.Assert(t, fsConfigRenderer.Render(ss))
	customOutputPath = fsConfigRenderer.GetOutputPath(ss)
	for _, dirName := range []string{"certificates", "config", "sources", "runtime"} {
		file, err := os.Stat(path.Join(customOutputPath, dirName))
		assert.Assert(t, err)
		assert.Assert(t, file.IsDir())
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
		"runtime/token/link-link-access-one.yaml",
	}
	for _, fileName := range expectedFiles {
		fs, err := os.Stat(path.Join(customOutputPath, fileName))
		assert.Assert(t, err)
		assert.Assert(t, fs.Mode().IsRegular())
		assert.Assert(t, fs.Size() > 0)
	}
}

func fakeSiteState() *api.SiteState {
	return &api.SiteState{
		SiteId: "site-id",
		Site: &v1alpha1.Site{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Site",
				APIVersion: "skupper.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "site-name",
			},
			Spec: v1alpha1.SiteSpec{},
		},
		Listeners: map[string]*v1alpha1.Listener{
			"listener-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Listener",
					APIVersion: "skupper.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "listener-one",
				},
				Spec: v1alpha1.ListenerSpec{
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
					APIVersion: "skupper.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "listener-two",
				},
				Spec: v1alpha1.ListenerSpec{
					RoutingKey:     "listener-two-key",
					Host:           "10.0.0.2",
					Port:           1234,
					TlsCredentials: "listener-two-credentials",
					Type:           "tcp",
				},
			},
		},
		Connectors: map[string]*v1alpha1.Connector{
			"connector-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Connector",
					APIVersion: "skupper.io/v1alpha1",
				},
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
				TypeMeta: metav1.TypeMeta{
					Kind:       "RouterAccess",
					APIVersion: "skupper.io/v1alpha1",
				},
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
		},
		Grants: make(map[string]*v1alpha1.AccessGrant),
		Links: map[string]*v1alpha1.Link{
			"link-one": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Link",
					APIVersion: "skupper.io/v1alpha1",
				},
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
		Claims:          make(map[string]*v1alpha1.AccessToken),
		Certificates:    make(map[string]*v1alpha1.Certificate),
		SecuredAccesses: make(map[string]*v1alpha1.SecuredAccess),
	}
}
