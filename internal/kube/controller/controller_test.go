package controller

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"testing"

	"gotest.tools/v3/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/resource"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type WaitFunction func(t *testing.T, clients internalclient.Clients) bool

func TestGeneral(t *testing.T) {
	testTable := []struct {
		name                                     string
		args                                     []string
		env                                      map[string]string
		k8sObjects                               []runtime.Object
		skupperObjects                           []runtime.Object
		waitFunctions                            []WaitFunction
		expectedDynamicResources                 map[schema.GroupVersionResource]*unstructured.Unstructured
		expectedRouterConfig                     []*RouterConfig
		expectedServices                         []*corev1.Service
		expectedRouterAccesses                   []*skupperv2alpha1.RouterAccess
		expectedSecuredAccesses                  []*skupperv2alpha1.SecuredAccess
		expectedSiteStatuses                     []*skupperv2alpha1.Site
		expectedListenerStatuses                 []*skupperv2alpha1.Listener
		expectedConnectorStatuses                []*skupperv2alpha1.Connector
		expectedAttachedConnectorStatuses        []*skupperv2alpha1.AttachedConnector
		expectedAttachedConnectorBindingStatuses []*skupperv2alpha1.AttachedConnectorBinding
	}{
		{
			name: "simple site",
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysite", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
		},
		{
			name: "site with running router pods",
			k8sObjects: []runtime.Object{
				f.pod("skupper-router-1", "test", map[string]string{"skupper.io/component": "router", "skupper.io/type": "site"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysite", "test", skupperv2alpha1.StatusReady, "OK", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK"),
					f.condition(skupperv2alpha1.CONDITION_TYPE_RUNNING, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
		},
		{
			name: "site with network status",
			k8sObjects: []runtime.Object{
				f.skupperNetworkStatus("test", f.networkStatusInfo("test")),
				f.pod("skupper-router-1", "test", map[string]string{"skupper.io/component": "router", "skupper.io/type": "site"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
			},
			waitFunctions: []WaitFunction{
				isSiteStatusConditionTrue("mysite", "test", skupperv2alpha1.CONDITION_TYPE_RUNNING),
				isSiteNetworkStatusSet("mysite", "test"),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.addNetworkStatus(f.siteStatus("mysite", "test", skupperv2alpha1.StatusReady, "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
		},
		{
			name: "site with default link access",
			k8sObjects: []runtime.Object{
				f.pod("skupper-router-1", "test", map[string]string{"skupper.io/component": "router", "skupper.io/type": "site"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "default", false, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Pending", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK"),
					f.condition(skupperv2alpha1.CONDITION_TYPE_RUNNING, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedRouterAccesses: []*skupperv2alpha1.RouterAccess{
				f.routerAccess("skupper-router", "test", "", "skupper-site-server", true, "skupper-site-ca", f.role("inter-router", 55671), f.role("edge", 45671)),
			},
		},
		{
			name: "site with specific link access",
			env: map[string]string{
				"SKUPPER_ENABLED_ACCESS_TYPES": "route,loadbalancer,ingress-nginx",
			},
			k8sObjects: []runtime.Object{
				f.pod("skupper-router-1", "test", map[string]string{"skupper.io/component": "router", "skupper.io/type": "site"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "ingress-nginx", false, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Pending", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK"),
					f.condition(skupperv2alpha1.CONDITION_TYPE_RUNNING, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedRouterAccesses: []*skupperv2alpha1.RouterAccess{
				f.routerAccess("skupper-router", "test", "ingress-nginx", "skupper-site-server", true, "skupper-site-ca", f.role("inter-router", 55671), f.role("edge", 45671)),
			},
			expectedSecuredAccesses: []*skupperv2alpha1.SecuredAccess{
				f.securedAccess("skupper-router", "test", "ingress-nginx", f.routerSelectorWithGroup("skupper-router"), "skupper-site-server", "skupper-site-ca",
					f.securedAccessPort("inter-router", 55671), f.securedAccessPort("edge", 45671)),
			},
		},
		{
			name: "ha site with default link access",
			k8sObjects: []runtime.Object{
				f.pod("skupper-router-1", "test", map[string]string{"skupper.io/component": "router", "skupper.io/type": "site"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "default", true, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Pending", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK"),
					f.condition(skupperv2alpha1.CONDITION_TYPE_RUNNING, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
				resource.DeploymentResource(): f.routerDeployment("skupper-router-2", "test"),
			},
			expectedRouterAccesses: []*skupperv2alpha1.RouterAccess{
				f.routerAccess("skupper-router", "test", "", "skupper-site-server", true, "skupper-site-ca", f.role("inter-router", 55671), f.role("edge", 45671)),
			},
			expectedSecuredAccesses: []*skupperv2alpha1.SecuredAccess{
				f.securedAccess("skupper-router", "test", "", f.routerSelectorWithGroup("skupper-router"), "skupper-site-server", "skupper-site-ca",
					f.securedAccessPort("inter-router", 55671), f.securedAccessPort("edge", 45671)),
				f.securedAccess("skupper-router-2", "test", "", f.routerSelectorWithGroup("skupper-router-2"), "skupper-site-server", "skupper-site-ca",
					f.securedAccessPort("inter-router", 55671), f.securedAccessPort("edge", 45671)),
			},
		},
		{
			name: "site with router access",
			env: map[string]string{
				"SKUPPER_ENABLED_ACCESS_TYPES": "route,loadbalancer,ingress-nginx",
			},
			k8sObjects: []runtime.Object{
				f.pod("skupper-router-1", "test", map[string]string{"skupper.io/component": "router", "skupper.io/type": "site"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "ingress-nginx", false, false),
				f.routerAccess("foo", "test", "ingress-nginx", "foo-server", true, "foo-ca", f.role("inter-router", 55555)),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Pending", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK"),
					f.condition(skupperv2alpha1.CONDITION_TYPE_RUNNING, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedSecuredAccesses: []*skupperv2alpha1.SecuredAccess{
				f.securedAccess("foo", "test", "ingress-nginx", f.routerSelectorWithGroup("skupper-router"), "foo-server", "foo-ca",
					f.securedAccessPort("inter-router", 55555)),
			},
		},
		{
			name: "site with listener",
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "", false, false),
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedRouterConfig: []*RouterConfig{
				f.routerConfig("skupper-router", "test").tcpListener("mylistener", "1024", ""),
			},
			expectedServices: []*corev1.Service{
				f.service("mysvc", "test", f.routerSelector(true), f.servicePort("mylistener", 8080, 1024)),
			},
		},
		{
			name: "listener with no site",
			skupperObjects: []runtime.Object{
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			expectedListenerStatuses: []*skupperv2alpha1.Listener{
				f.listenerStatus("mylistener", "test", skupperv2alpha1.StatusError, "No active site in namespace",
					f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse, "Error", "No active site in namespace")),
			},
		},
		{
			name: "site and connector with host field",
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "", false, false),
				f.connector("myconnector", "test", "mysvc", 8080),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedRouterConfig: []*RouterConfig{
				f.routerConfig("skupper-router", "test").tcpConnector("myconnector@mysvc", "mysvc", "8080", ""),
			},
		},
		{
			name: "site and connector with selector field",
			k8sObjects: []runtime.Object{
				f.pod("mypod-1", "test", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
				f.pod("mypod-2", "test", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.11", corev1.PodPending)), //not running
				f.pod("mypod-3", "test", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.12", corev1.PodRunning)), //not ready
			},
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "", false, false),
				f.connectorWithSelector("myconnector", "test", "app=foo", 8080),
			},
			waitFunctions: []WaitFunction{
				isConnectorStatusConditionTrue("myconnector", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedRouterConfig: []*RouterConfig{
				f.routerConfig("skupper-router", "test").tcpConnector("myconnector@10.1.1.10", "10.1.1.10", "8080", ""),
			},
		},
		{
			name: "connector with no site",
			skupperObjects: []runtime.Object{
				f.connector("myconnector", "test", "mysvc", 8080),
			},
			expectedConnectorStatuses: []*skupperv2alpha1.Connector{
				f.connectorStatus("myconnector", "test", skupperv2alpha1.StatusError, "No active site in namespace",
					f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse, "Error", "No active site in namespace")),
			},
		},
		{
			name: "site with attached connector",
			k8sObjects: []runtime.Object{
				f.pod("mypod-1", "other", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
				f.pod("mypod-2", "other", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.11", corev1.PodPending)), //not running
				f.pod("mypod-3", "other", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.12", corev1.PodRunning)), //not ready
			},
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "", false, false),
				f.attachedConnector("myconnector", "other", "test", "app=foo", 8080),
				f.attachedConnectorBinding("myconnector", "test", "other"),
			},
			waitFunctions: []WaitFunction{
				isAttachedConnectorStatusConditionTrue("myconnector", "other", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysvc", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedRouterConfig: []*RouterConfig{
				f.routerConfig("skupper-router", "test").tcpConnector("myconnector@10.1.1.10", "10.1.1.10", "8080", ""),
			},
		},
		/* TODO: Fix
		{
			name: "attached connector with no site",
			k8sObjects: []runtime.Object{
				f.pod("mypod-1", "other", map[string]string{"app":"foo"}, nil, f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
				f.pod("mypod-2", "other", map[string]string{"app":"foo"}, nil, f.podStatus("10.1.1.11", corev1.PodPending)),//not running
				f.pod("mypod-3", "other", map[string]string{"app":"foo"}, nil, f.podStatus("10.1.1.12", corev1.PodRunning)),//not ready
			},
			skupperObjects: []runtime.Object{
				f.attachedConnector("myconnector", "other", "test", "app=foo", 8080),
				f.attachedConnectorBinding("myconnector", "test", "other"),
			},
			expectedAttachedConnectorBindingStatuses: []*skupperv2alpha1.AttachedConnectorBinding{
				f.attachedConnectorBindingStatus("myconnector", "test", skupperv2alpha1.StatusError, "No active site in namespace",
					f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse, "Error", "No active site in namespace")),
			},
		},
		*/
		{
			name: "attached connector mismatch",
			k8sObjects: []runtime.Object{
				f.pod("mypod-1", "other", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysvc", "test", "", false, false),
				f.attachedConnector("foo", "other", "test", "app=foo", 8080),
				f.attachedConnectorBinding("bar", "test", "other"),
			},
			expectedAttachedConnectorStatuses: []*skupperv2alpha1.AttachedConnector{
				f.attachedConnectorStatus("foo", "other", skupperv2alpha1.StatusError, "No matching AttachedConnectorBinding in site namespace",
					f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse, "Error", "No matching AttachedConnectorBinding in site namespace")),
			},
			expectedAttachedConnectorBindingStatuses: []*skupperv2alpha1.AttachedConnectorBinding{
				f.attachedConnectorBindingStatus("bar", "test", skupperv2alpha1.StatusError, "No matching AttachedConnector",
					f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse, "Error", "No matching AttachedConnector")),
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			flags := &flag.FlagSet{}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			config, err := BoundConfig(flags)
			assert.Assert(t, err)
			flags.Parse(tt.args)
			clients, err := fakeclient.NewFakeClient(config.Namespace, tt.k8sObjects, tt.skupperObjects, "")
			assert.Assert(t, err)
			enableSSA(clients.GetDynamicClient())
			controller, err := NewController(clients, config)
			assert.Assert(t, err)
			stopCh := make(chan struct{})
			err = controller.init(stopCh)
			assert.Assert(t, err)
			for i := 0; i < len(tt.k8sObjects)+len(tt.skupperObjects); i++ {
				controller.controller.TestProcess()
			}
			for _, wf := range tt.waitFunctions {
				for !wf(t, clients) {
					controller.controller.TestProcess()
				}
			}
			for _, expected := range tt.expectedSiteStatuses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().Sites(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				verifyStatus(t, expected.Status.Status, actual.Status.Status)
				if expected.Status.Network != nil {
					assert.DeepEqual(t, expected.Status.Network, actual.Status.Network)
				}
			}
			for _, expected := range tt.expectedListenerStatuses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().Listeners(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				verifyStatus(t, expected.Status.Status, actual.Status.Status)
			}
			for _, expected := range tt.expectedConnectorStatuses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().Connectors(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				verifyStatus(t, expected.Status.Status, actual.Status.Status)
			}
			for _, expected := range tt.expectedAttachedConnectorStatuses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectors(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				verifyStatus(t, expected.Status.Status, actual.Status.Status)
			}
			for _, expected := range tt.expectedAttachedConnectorBindingStatuses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectorBindings(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				verifyStatus(t, expected.Status.Status, actual.Status.Status)
			}
			for resource, expected := range tt.expectedDynamicResources {
				actual, err := clients.GetDynamicClient().Resource(resource).Namespace(expected.GetNamespace()).Get(context.Background(), expected.GetName(), metav1.GetOptions{})
				assert.Assert(t, err)
				actualLabels := actual.GetLabels()
				for key, expectedValue := range expected.GetLabels() {
					actualValue, ok := actualLabels[key]
					assert.Assert(t, ok, key)
					assert.Equal(t, expectedValue, actualValue)
				}
				match(expected.UnstructuredContent(), actual.UnstructuredContent())
			}
			for _, expected := range tt.expectedRouterConfig {
				actual, err := clients.GetKubeClient().CoreV1().ConfigMaps(expected.namespace).Get(context.Background(), expected.name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.Assert(t, expected.verify(t, actual))
			}
			for _, expected := range tt.expectedServices {
				actual, err := clients.GetKubeClient().CoreV1().Services(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.DeepEqual(t, expected.Spec.Selector, actual.Spec.Selector)
				assert.DeepEqual(t, expected.Spec.Ports, actual.Spec.Ports)
			}
			for _, expected := range tt.expectedRouterAccesses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().RouterAccesses(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.DeepEqual(t, expected.Spec, actual.Spec)
			}
			for _, expected := range tt.expectedSecuredAccesses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				assert.DeepEqual(t, expected.Spec, actual.Spec)
			}
		})
	}
}

func verifyStatus(t *testing.T, expected skupperv2alpha1.Status, actual skupperv2alpha1.Status) {
	assert.Equal(t, expected.StatusType, actual.StatusType)
	assert.Equal(t, expected.Message, actual.Message)
	for _, condition := range expected.Conditions {
		existing := meta.FindStatusCondition(actual.Conditions, condition.Type)
		assert.Assert(t, existing != nil)
		assert.Equal(t, condition.Status, existing.Status)
		assert.Equal(t, condition.Reason, existing.Reason)
		if condition.Message != "" {
			assert.Equal(t, condition.Message, existing.Message)
		}
	}
}

func enableSSA(client dynamic.Interface) bool {
	if fc, ok := client.(*fakedynamic.FakeDynamicClient); ok {
		fc.PrependReactor(
			"patch",
			"*",
			func(action k8stesting.Action) (bool, runtime.Object, error) {
				pa := action.(k8stesting.PatchAction)
				if pa.GetPatchType() != types.ApplyPatchType {
					return false, nil, nil
				}
				// Apply patches are supposed to upsert, but fake client fails if the object doesn't exist,
				// if an apply patch occurs for a deployment that doesn't yet exist, create it.
				// However, we already hold the fakeclient lock, so we can't use the front door.
				rfunc := k8stesting.ObjectReaction(fc.Tracker())
				_, obj, err := rfunc(
					k8stesting.NewGetAction(pa.GetResource(), pa.GetNamespace(), pa.GetName()),
				)
				if errors.IsNotFound(err) || obj == nil {
					_, _, _ = rfunc(
						k8stesting.NewCreateAction(
							pa.GetResource(),
							pa.GetNamespace(),
							&appsv1.Deployment{
								ObjectMeta: metav1.ObjectMeta{
									Name:      pa.GetName(),
									Namespace: pa.GetNamespace(),
								},
							},
						),
					)
				}
				return rfunc(k8stesting.NewPatchAction(
					pa.GetResource(),
					pa.GetNamespace(),
					pa.GetName(),
					types.StrategicMergePatchType,
					pa.GetPatch()))
			},
		)
		return true
	}
	return false
}

type factory struct{}

func (*factory) routerConfig(name string, namespace string) *RouterConfig {
	return &RouterConfig{
		name:      name,
		namespace: namespace,
		config: &qdr.RouterConfig{
			Addresses:   map[string]qdr.Address{},
			SslProfiles: map[string]qdr.SslProfile{},
			Listeners:   map[string]qdr.Listener{},
			Connectors:  map[string]qdr.Connector{},
			LogConfig:   map[string]qdr.LogConfig{},
			Bridges: qdr.BridgeConfig{
				TcpListeners:  map[string]qdr.TcpEndpoint{},
				TcpConnectors: map[string]qdr.TcpEndpoint{},
			},
		},
	}
}

func (*factory) site(name string, namespace string, linkAccess string, ha bool, edge bool) *skupperv2alpha1.Site {
	return &skupperv2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.SiteSpec{
			HA:         ha,
			LinkAccess: linkAccess,
			Edge:       edge,
		},
	}
}

func (*factory) listener(name string, namespace string, host string, port int) *skupperv2alpha1.Listener {
	return &skupperv2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.ListenerSpec{
			Host: host,
			Port: port,
		},
	}
}

func (*factory) connector(name string, namespace string, host string, port int) *skupperv2alpha1.Connector {
	return &skupperv2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.ConnectorSpec{
			Host: host,
			Port: port,
		},
	}
}

func (*factory) connectorWithSelector(name string, namespace string, selector string, port int) *skupperv2alpha1.Connector {
	return &skupperv2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.ConnectorSpec{
			Selector: selector,
			Port:     port,
		},
	}
}

func (*factory) attachedConnector(name string, namespace string, siteNamespace string, selector string, port int) *skupperv2alpha1.AttachedConnector {
	return &skupperv2alpha1.AttachedConnector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AttachedConnector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.AttachedConnectorSpec{
			SiteNamespace: siteNamespace,
			Selector:      selector,
			Port:          port,
		},
	}
}

func (*factory) attachedConnectorBinding(name string, namespace string, connectorNamespace string) *skupperv2alpha1.AttachedConnectorBinding {
	return &skupperv2alpha1.AttachedConnectorBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AttachedConnectorBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.AttachedConnectorBindingSpec{
			ConnectorNamespace: connectorNamespace,
		},
	}
}

func (*factory) siteStatus(name string, namespace string, statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) *skupperv2alpha1.Site {
	return &skupperv2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: skupperv2alpha1.SiteStatus{
			Status: f.status(statusType, message, conditions...),
		},
	}
}

func (*factory) listenerStatus(name string, namespace string, statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) *skupperv2alpha1.Listener {
	return &skupperv2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: skupperv2alpha1.ListenerStatus{
			Status: f.status(statusType, message, conditions...),
		},
	}
}

func (*factory) connectorStatus(name string, namespace string, statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) *skupperv2alpha1.Connector {
	return &skupperv2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: skupperv2alpha1.ConnectorStatus{
			Status: f.status(statusType, message, conditions...),
		},
	}
}

func (*factory) attachedConnectorStatus(name string, namespace string, statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) *skupperv2alpha1.AttachedConnector {
	return &skupperv2alpha1.AttachedConnector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AttachedConnector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: skupperv2alpha1.AttachedConnectorStatus{
			Status: f.status(statusType, message, conditions...),
		},
	}
}

func (*factory) attachedConnectorBindingStatus(name string, namespace string, statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) *skupperv2alpha1.AttachedConnectorBinding {
	return &skupperv2alpha1.AttachedConnectorBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AttachedConnectorBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: skupperv2alpha1.AttachedConnectorBindingStatus{
			Status: f.status(statusType, message, conditions...),
		},
	}
}

func (*factory) status(statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) skupperv2alpha1.Status {
	return skupperv2alpha1.Status{
		StatusType: statusType,
		Message:    message,
		Conditions: conditions,
	}
}

func (*factory) condition(conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func (*factory) routerSelector(application bool) map[string]string {
	selector := map[string]string{
		"skupper.io/component": "router",
	}
	if application {
		selector["application"] = "skupper-router"
	}
	return selector
}

func (*factory) routerSelectorWithGroup(group string) map[string]string {
	selector := map[string]string{
		"skupper.io/component": "router",
		"skupper.io/group":     group,
	}
	return selector
}

func (f *factory) routerDeployment(name string, namespace string) *unstructured.Unstructured {
	labels := map[string]string{
		"application":          "skupper-router",
		"skupper.io/component": "router",
		"skupper.io/type":      "site",
	}
	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": f.routerSelector(false),
			},
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name": "router",
						},
						{
							"name": "kube-adaptor",
						},
					},
				},
			},
		},
	}
	return f.unstructured(name, namespace, content, labels)
}

func (*factory) unstructured(name string, namespace string, content map[string]interface{}, labels map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	if content != nil {
		obj.SetUnstructuredContent(content)
	}
	obj.SetName(name)
	obj.SetNamespace(namespace)
	if labels != nil {
		obj.SetLabels(labels)
	}
	return obj
}

func (*factory) routerAccess(name string, namespace string, accessType string, credentials string, generate bool, issuer string, roles ...skupperv2alpha1.RouterAccessRole) *skupperv2alpha1.RouterAccess {
	return &skupperv2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.RouterAccessSpec{
			AccessType:             accessType,
			TlsCredentials:         credentials,
			GenerateTlsCredentials: generate,
			Issuer:                 issuer,
			Roles:                  roles,
		},
	}
}

func (*factory) role(name string, port int) skupperv2alpha1.RouterAccessRole {
	return skupperv2alpha1.RouterAccessRole{
		Name: name,
		Port: port,
	}
}

func (*factory) securedAccess(name string, namespace string, accessType string, selector map[string]string, certificate string, issuer string, ports ...skupperv2alpha1.SecuredAccessPort) *skupperv2alpha1.SecuredAccess {
	return &skupperv2alpha1.SecuredAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "SecuredAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: skupperv2alpha1.SecuredAccessSpec{
			AccessType:  accessType,
			Selector:    selector,
			Certificate: certificate,
			Issuer:      issuer,
			Ports:       ports,
		},
	}
}

func (*factory) securedAccessPort(name string, port int) skupperv2alpha1.SecuredAccessPort {
	return skupperv2alpha1.SecuredAccessPort{
		Name:       name,
		Port:       port,
		TargetPort: port,
		Protocol:   "TCP",
	}
}

func (*factory) service(name string, namespace string, selector map[string]string, ports ...corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    ports,
		},
	}
}

func (*factory) servicePort(name string, port int32, targetPort int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.IntOrString{IntVal: int32(targetPort)},
		Protocol:   corev1.Protocol("TCP"),
	}
}

func (*factory) pod(name string, namespace string, labels map[string]string, ownerRefs []metav1.OwnerReference, status *corev1.PodStatus) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
	}
	if status != nil {
		pod.Status = *status
	}
	return pod
}

func (*factory) podStatus(ip string, phase corev1.PodPhase, conditions ...corev1.PodCondition) *corev1.PodStatus {
	return &corev1.PodStatus{
		Phase:      phase,
		Conditions: conditions,
		PodIP:      ip,
	}
}

func (*factory) podCondition(conditionType corev1.PodConditionType, status corev1.ConditionStatus) corev1.PodCondition {
	return corev1.PodCondition{
		Type:   conditionType,
		Status: status,
	}
}

func (*factory) addNetworkStatus(site *skupperv2alpha1.Site) *skupperv2alpha1.Site {
	//add equivalent data to that generated in networkStatusInfo
	site.Status.Network = []skupperv2alpha1.SiteRecord{
		{
			Id:        "site-1",
			Name:      "east",
			Namespace: site.Namespace,
			Services: []skupperv2alpha1.ServiceRecord{
				{
					RoutingKey: "mysvc",
					Listeners: []string{
						"mysvc",
					},
					Connectors: []string{
						"foo",
					},
				},
			},
		},
		{
			Id:        "site-2",
			Name:      "west",
			Namespace: "other",
			Links: []skupperv2alpha1.LinkRecord{
				{
					Name:           "link1",
					RemoteSiteId:   "site-1",
					RemoteSiteName: "east",
				},
			},
		},
	}
	return site
}

func (*factory) skupperNetworkStatus(namespace string, status *network.NetworkStatusInfo) *corev1.ConfigMap {
	data, _ := json.Marshal(status)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "skupper-network-status",
			Namespace: namespace,
		},
		Data: map[string]string{
			"NetworkStatus": string(data),
		},
	}
}

func (*factory) networkStatusInfo(namespace string) *network.NetworkStatusInfo {
	//create some dummy data
	return &network.NetworkStatusInfo{
		SiteStatus: []network.SiteStatusInfo{
			{
				Site: network.SiteInfo{
					Identity:  "site-1",
					Name:      "east",
					Namespace: namespace,
				},
				RouterStatus: []network.RouterStatusInfo{
					{
						Router: network.RouterInfo{
							Name:      "red-skupper-router-5945f87d48-j97sp",
							Namespace: namespace,
						},
						AccessPoints: []network.RouterAccessInfo{
							{
								Identity: "ap-site-1",
							},
						},
						Listeners: []network.ListenerInfo{
							{
								Name:    "mysvc",
								Address: "mysvc",
							},
						},
						Connectors: []network.ConnectorInfo{
							{
								Address:  "mysvc",
								DestHost: "foo",
							},
						},
					},
				},
			},
			{
				Site: network.SiteInfo{
					Identity:  "site-2",
					Name:      "west",
					Namespace: "other",
				},
				RouterStatus: []network.RouterStatusInfo{
					{
						Router: network.RouterInfo{
							Name:      "blue-skupper-router-6435f81d22-a2xtp",
							Namespace: "other",
						},
						Links: []network.LinkInfo{
							{
								Name: "link1",
								Peer: "ap-site-1",
							},
						},
					},
				},
			},
		},
	}
}

var f factory

// ensures all fields specified in expected are in actual
func match(expected interface{}, actual interface{}) error {
	return _match(reflect.ValueOf(expected), reflect.ValueOf(actual))
}

func _match(expected reflect.Value, actual reflect.Value) error {
	if expected.Kind() != actual.Kind() {
		return fmt.Errorf("Expected value of type %s, got value of type %s", expected.Kind(), actual.Kind())
	} else if expected.Kind() == reflect.Map {
		for _, key := range expected.MapKeys() {
			if err := _match(expected.MapIndex(key), actual.MapIndex(key)); err != nil {
				return fmt.Errorf("Value for %s does not match: %s", key.String(), err)
			}
		}
		return nil
	} else if expected.Kind() == reflect.Slice || expected.Kind() == reflect.Array {
		if expected.Len() != actual.Len() {
			return fmt.Errorf("Different number of items, expected %s, got %s", expected.String(), actual.String())
		}
		for i := 0; i < expected.Len(); i++ {
			if err := _match(expected.Index(i), actual.Index(i)); err != nil {
				return fmt.Errorf("Item at index %d does not match: %s", i, err)
			}
		}
		return nil
	} else if expected.Comparable() {
		if !expected.Equal(actual) {
			return fmt.Errorf("Expected %s, got  %s", expected.String(), actual.String())
		}
		return nil
	} else {
		return fmt.Errorf("Type %s is not comparable", expected.Kind())
	}
}

type RouterConfig struct {
	namespace string
	name      string
	config    *qdr.RouterConfig
}

func (rc *RouterConfig) tcpListener(name string, port string, sslProfile string) *RouterConfig {
	rc.config.Bridges.TcpListeners[name] = qdr.TcpEndpoint{
		Name:       name,
		Port:       port,
		SslProfile: sslProfile,
	}
	return rc
}

func (rc *RouterConfig) tcpConnector(name string, host string, port string, sslProfile string) *RouterConfig {
	rc.config.Bridges.TcpConnectors[name] = qdr.TcpEndpoint{
		Name:       name,
		Host:       host,
		Port:       port,
		SslProfile: sslProfile,
	}
	return rc
}

func (rc *RouterConfig) verify(t *testing.T, cm *corev1.ConfigMap) error {
	config, err := qdr.GetRouterConfigFromConfigMap(cm)
	assert.Assert(t, err)
	for key, expected := range rc.config.Addresses {
		actual, ok := config.Addresses[key]
		assert.Assert(t, ok, "No address found for %s", key)
		assert.Equal(t, actual, expected)
	}
	for key, expected := range rc.config.SslProfiles {
		actual, ok := config.SslProfiles[key]
		assert.Assert(t, ok, "No ssl profile found for %s", key)
		assert.Equal(t, actual, expected)
	}
	for key, expected := range rc.config.Listeners {
		actual, ok := config.Listeners[key]
		assert.Assert(t, ok, "No listener found for %s", key)
		assert.Equal(t, actual, expected)
	}
	for key, expected := range rc.config.Connectors {
		actual, ok := config.Connectors[key]
		assert.Assert(t, ok, "No connector found for %s", key)
		assert.Equal(t, actual, expected)
	}
	for key, expected := range rc.config.Bridges.TcpListeners {
		actual, ok := config.Bridges.TcpListeners[key]
		assert.Assert(t, ok, "No tcp listener found for %s", key)
		assert.Equal(t, actual, expected)
	}
	for key, expected := range rc.config.Bridges.TcpConnectors {
		actual, ok := config.Bridges.TcpConnectors[key]
		assert.Assert(t, ok, "No tcp connector found for %s", key)
		assert.Equal(t, actual, expected)
	}
	// for bridges, if specify at least one, specify them all:
	if len(rc.config.Bridges.TcpListeners) > 0 {
		assert.Equal(t, len(rc.config.Bridges.TcpListeners), len(config.Bridges.TcpListeners))
	}
	if len(rc.config.Bridges.TcpConnectors) > 0 {
		assert.Equal(t, len(rc.config.Bridges.TcpConnectors), len(config.Bridges.TcpConnectors))
	}
	return nil
}

func isSiteStatusConditionTrue(name string, namespace string, condition string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		connector, err := clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return meta.IsStatusConditionTrue(connector.Status.Conditions, condition)
	}
}

func isSiteNetworkStatusSet(name string, namespace string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		connector, err := clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return connector.Status.Network != nil
	}
}

func isConnectorStatusConditionTrue(name string, namespace string, condition string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		connector, err := clients.GetSkupperClient().SkupperV2alpha1().Connectors(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return meta.IsStatusConditionTrue(connector.Status.Conditions, condition)
	}
}

func isAttachedConnectorStatusConditionTrue(name string, namespace string, condition string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		connector, err := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectors(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return meta.IsStatusConditionTrue(connector.Status.Conditions, condition)
	}
}
