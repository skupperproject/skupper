package site

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	testingk8s "k8s.io/client-go/testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type TestClient struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (c *TestClient) GetKubeClient() kubernetes.Interface {
	return c.KubeClient
}

func (*TestClient) GetDynamicClient() dynamic.Interface {
	return nil
}

func (*TestClient) GetDiscoveryClient() *discovery.DiscoveryClient {
	return nil
}

func (*TestClient) GetRouteClient() *routev1client.RouteV1Client {
	return nil
}

func (*TestClient) VerifySiteCompatibility(siteVersion string) error {
	if siteVersion == "this-site-is-no-good" {
		return fmt.Errorf("Incompatible site")
	}
	return nil
}

func TestSiteContext(t *testing.T) {
	event.StartDefaultEventStore(nil)
	var tests = []struct {
		name            string
		options         map[string]string
		version         string
		edge            bool
		localAccessOnly bool
		edgeAddr        resolver.HostPort
		interRouterAddr resolver.HostPort
		claimsAddr      resolver.HostPort
		hosts           []string
		noSiteCfg       bool
		noRouterCfg     bool
		badRouterCfg    bool
		getCmError      error
		expectError     bool
	}{
		{
			name:            "foo",
			version:         "def",
			options:         map[string]string{"ingress": "none"},
			localAccessOnly: true,
			edgeAddr: resolver.HostPort{
				Host: "skupper-router.test-site-context-foo",
				Port: 45671,
			},
			interRouterAddr: resolver.HostPort{
				Host: "skupper-router.test-site-context-foo",
				Port: 55671,
			},
			claimsAddr: resolver.HostPort{
				Host: "skupper-router.test-site-context-foo",
				Port: 8081,
			},
			hosts: []string{
				"skupper-router.test-site-context-foo",
			},
		},
		{
			name:    "bar",
			version: "ghi",
			edge:    true,
		},
		{
			name:      "fail1",
			version:   "ghi",
			noSiteCfg: true,
		},
		{
			name:        "fail2",
			version:     "ghi",
			noRouterCfg: true,
		},
		{
			name:         "fail3",
			version:      "ghi",
			badRouterCfg: true,
		},
		{
			name:       "fail4",
			version:    "ghi",
			getCmError: fmt.Errorf("Error getting configmap"),
		},
		{
			name:        "fail5",
			options:     map[string]string{"ingress": "badvalue"},
			version:     "ghi",
			expectError: true,
		},
	}
	for _, test := range tests {
		cli := &TestClient{
			Namespace:  "test-site-context-" + test.name,
			KubeClient: fake.NewSimpleClientset(),
		}
		var err error
		var scm, rcm *corev1.ConfigMap
		//create site config
		if !test.noSiteCfg {
			scm, err = cli.createConfigMap(types.SiteConfigMapName, test.options)
			assert.Check(t, err)
		}
		//create router config
		if test.badRouterCfg {
			rcm, err = cli.createConfigMap(types.TransportConfigMapName, map[string]string{types.TransportConfigFile: "badformat"})
			assert.Check(t, err)
		} else if !test.noRouterCfg {
			rcfg := qdr.InitialConfig(test.name, test.name, test.version, test.edge, 0)
			rdata, err := rcfg.AsConfigMapData()
			assert.Check(t, err)
			rcm, err = cli.createConfigMap(types.TransportConfigMapName, rdata)
			assert.Check(t, err)
		}

		if test.getCmError != nil {
			cli.KubeClient.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("get", "configmaps", func(action testingk8s.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, test.getCmError
			})
		}

		ctxt, err := GetSiteContext(cli, cli.Namespace, context.TODO())
		if test.noSiteCfg || test.noRouterCfg || test.badRouterCfg || test.expectError {
			assert.Assert(t, err != nil)
			continue
		} else if test.getCmError != nil {
			assert.Equal(t, err, test.getCmError)
			continue
		}

		assert.Check(t, err)
		assert.Equal(t, ctxt.GetSiteId(), string(scm.ObjectMeta.UID))
		assert.Equal(t, ctxt.GetSiteVersion(), test.version)
		assert.Equal(t, ctxt.IsEdge(), test.edge)
		assert.Assert(t, reflect.DeepEqual(ctxt.GetOwnerReferences(), rcm.ObjectMeta.OwnerReferences))
		if !test.edge {
			assert.Equal(t, ctxt.IsLocalAccessOnly(), test.localAccessOnly)
			addr, err := ctxt.GetHostPortForEdge()
			assert.Check(t, err)
			assert.Equal(t, addr, test.edgeAddr)
			addr, err = ctxt.GetHostPortForInterRouter()
			assert.Check(t, err)
			assert.Equal(t, addr, test.interRouterAddr)
			addr, err = ctxt.GetHostPortForClaims()
			assert.Check(t, err)
			assert.Equal(t, addr, test.claimsAddr)
			hosts, err := ctxt.GetAllHosts()
			assert.Check(t, err)
			assert.Assert(t, reflect.DeepEqual(hosts, test.hosts))
		}
	}
}

func (c *TestClient) createConfigMap(name string, data map[string]string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
	}
	return c.GetKubeClient().CoreV1().ConfigMaps(c.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
}
