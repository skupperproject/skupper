package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/yaml"
)

const (
	delaySecs = 1
	attempts  = 5
)

func TestNetworkStatusHandler(t *testing.T) {
	var err error
	var nsCtrl *NamespaceController
	var nsHandler *NetworkStatusHandler
	var logHandler *testLogHandler

	tempDir := t.TempDir()
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = tempDir
	} else {
		t.Setenv("XDG_DATA_HOME", tempDir)
	}
	namespacesPath := api.GetDefaultOutputNamespacesPath()
	assert.Assert(t, os.MkdirAll(namespacesPath, 0755))
	namespace := "test-network-status-handler"
	runtimePath := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)
	assert.Assert(t, os.MkdirAll(runtimePath, 0755))

	// Site state and network status
	siteState := fakeSiteState()
	networkStatus := fakeNetworkStatusInfo()

	t.Run("serialize-site-state", func(t *testing.T) {
		assert.Assert(t, api.MarshalSiteState(*siteState, runtimePath))
	})

	t.Run("start-network-status-handler", func(t *testing.T) {
		nsHandler = NewNetworkStatusHandler(namespace)
		nsCtrl, err = NewNamespaceController(namespace)
		assert.Assert(t, err)
		nsCtrl.prepare = func() {
			nsCtrl.watcher.Add(runtimePath, nsHandler)
		}

		// Custom Log Handler (intercept messages)
		logHandler = &testLogHandler{
			handler: nsHandler.logger.Handler(),
		}
		nsHandler.logger = slog.New(logHandler)

		nsCtrl.Start()
	})

	t.Run("update-network-status", func(t *testing.T) {
		cmHandler := fs.NewConfigMapHandler(namespace)
		cmStr, err := fakeConfigMap(networkStatus)
		assert.Assert(t, err)
		err = cmHandler.WriteFile(runtimePath, "skupper-network-status.yaml", cmStr, "ConfigMap")
		assert.Assert(t, err)
	})

	t.Run("verify-runtime-resources", func(t *testing.T) {
		assert.Assert(t, verifyJsonPathExpected(namespace, "Site", "site-name", "{.status.sitesInNetwork}", "2"))
		assert.Assert(t, verifyJsonPathExpected(namespace, "Connector", "connector-one", "{.status.hasMatchingListener}", "true"))
		assert.Assert(t, verifyJsonPathExpected(namespace, "Listener", "listener-one", "{.status.hasMatchingConnector}", "true"))
		assert.Assert(t, verifyJsonPathExpected(namespace, "Listener", "listener-one", "{.status.conditions[?(@.type=='Matched')].status}", "True"))
		assert.Assert(t, verifyJsonPathExpected(namespace, "Listener", "listener-two", "{.status.conditions[?(@.type=='Matched')].status}", "False"))
		assert.Assert(t, verifyJsonPathExpected(namespace, "Link", "link-one", "{.status.conditions[?(@.type=='Operational')].status}", "True"))
	})

	t.Run("remove-namespace", func(t *testing.T) {
		cmHandler := fs.NewConfigMapHandler(namespace)
		assert.Assert(t, cmHandler.Delete("skupper-network-status", true))
	})

	t.Run("verify-runtime-resources-reset", func(t *testing.T) {
		assert.Assert(t, verifyJsonPathNotFound(namespace, "Site", "site-name", "{.status.sitesInNetwork}"), "sitesInNetwork found, but not expected")
		assert.Assert(t, verifyJsonPathExpected(namespace, "Site", "site-name", "{.status.status}", "Pending"), "status expected as Pending")
		assert.Assert(t, verifyJsonPathNotFound(namespace, "Connector", "connector-one", "{.status.hasMatchingListener}"), "hasMatchingListener found, but not expected")
		assert.Assert(t, verifyJsonPathExpected(namespace, "Connector", "connector-one", "{.status.conditions[?(@.type=='Matched')].status}", "False"))
		assert.Assert(t, verifyJsonPathNotFound(namespace, "Listener", "listener-one", "{.status.hasMatchingConnector}"), "hasMatchingConnector found, but not expected")
		assert.Assert(t, verifyJsonPathExpected(namespace, "Listener", "listener-one", "{.status.conditions[?(@.type=='Matched')].status}", "False"))
		assert.Assert(t, verifyJsonPathExpected(namespace, "Listener", "listener-two", "{.status.conditions[?(@.type=='Matched')].status}", "False"))
		assert.Assert(t, verifyJsonPathExpected(namespace, "Link", "link-one", "{.status.conditions[?(@.type=='Operational')].status}", "False"))
	})

	t.Run("stop-network-status-handler", func(t *testing.T) {
		nsCtrl.Stop()
		err = utils.Retry(time.Second*delaySecs, attempts, func() (bool, error) {
			t.Log(logHandler.messages)
			return slices.Contains(logHandler.messages, "Stop event processing"), nil
		})
		assert.Assert(t, err)
	})

}

type testLogHandler struct {
	handler  slog.Handler
	messages []string
	mux      sync.Mutex
}

func (t *testLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (t *testLogHandler) Handle(ctx context.Context, record slog.Record) error {
	t.mux.Lock()
	defer t.mux.Unlock()
	t.messages = append(t.messages, record.Message)
	return t.handler.Handle(ctx, record)
}

func (t *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return t.handler.WithAttrs(attrs)
}

func (t *testLogHandler) WithGroup(name string) slog.Handler {
	return t.handler.WithGroup(name)
}

func (t *testLogHandler) Count(message string) int {
	t.mux.Lock()
	defer t.mux.Unlock()
	count := 0
	for _, msg := range t.messages {
		if msg == message {
			count++
		}
	}
	return count
}

func verifyJsonPathExpected(namespace, kind, name, jsonPath, expected string) error {
	verifyExpected := func(s string, err error) (bool, error) {
		if s != expected {
			return false, nil
		}
		return true, nil
	}
	return verifyJsonPath(namespace, kind, name, jsonPath, verifyExpected)
}

func verifyJsonPathNotFound(namespace, kind, name, jsonPath string) error {
	verifyExpected := func(s string, err error) (bool, error) {
		if err == nil {
			return false, nil
		}
		return strings.Contains(err.Error(), "is not found"), nil
	}
	return verifyJsonPath(namespace, kind, name, jsonPath, verifyExpected)
}

func verifyJsonPath(namespace, kind, name, jsonPath string, validate func(s string, err error) (bool, error)) error {
	return utils.Retry(time.Second*delaySecs, attempts, func() (bool, error) {
		var data interface{}
		buf := new(bytes.Buffer)

		fileName := kind + "-" + name + ".yaml"
		fqFileName := path.Join(api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath), fileName)
		f, err := os.Open(fqFileName)
		if err != nil {
			return validate("", err)
		}
		defer f.Close()

		err = utilyaml.NewYAMLToJSONDecoder(f).Decode(&data)
		if err != nil {
			return validate("", err)
		}

		j := jsonpath.New(jsonPath)
		err = j.Parse(jsonPath)
		if err != nil {
			return validate("", err)
		}

		err = j.Execute(buf, data)
		if err != nil {
			return validate("", err)
		}

		return validate(buf.String(), err)
	})
}

func fakeConfigMap(status network.NetworkStatusInfo) (string, error) {
	statusJson, err := json.Marshal(status)
	if err != nil {
		return "", err
	}
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "skupper-network-status",
		},
		Data: map[string]string{
			"NetworkStatus": string(statusJson),
		},
	}
	cmYaml, err := yaml.Marshal(cm)
	if err != nil {
		return "", err
	}
	return string(cmYaml), nil
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
					Host:           "listener-one-host",
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
					Host:           "listener-two-host",
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
			"skupper-local": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "RouterAccess",
					APIVersion: "skupper.io/v2alpha1",
				},
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
			"link-one-profile": {
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
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
		ConfigMaps:      map[string]*corev1.ConfigMap{},
		Claims:          make(map[string]*v2alpha1.AccessToken),
		Certificates:    make(map[string]*v2alpha1.Certificate),
		SecuredAccesses: make(map[string]*v2alpha1.SecuredAccess),
	}
}
