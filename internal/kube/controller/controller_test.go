package controller

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/internal/kube/watchers"
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
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/resource"
	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/version"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type WaitFunction func(t *testing.T, clients internalclient.Clients) bool
type ControllerFunction func(t *testing.T, controller *Controller) bool

func TestGeneral(t *testing.T) {
	fakeNetworkStatus := f.fakeNetworkStatusInfo("test")
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
			name: "ignored site",
			k8sObjects: []runtime.Object{
				f.configmap("skupper", "test", map[string]string{"controller": "foo/bar"}, nil, nil),
			},
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysite", "test", "", ""),
			},
		},
		{
			name: "explicitly matched site",
			env: map[string]string{
				"NAMESPACE":       "foo",
				"CONTROLLER_NAME": "bar",
			},
			k8sObjects: []runtime.Object{
				f.configmap("skupper", "test", map[string]string{"controller": "foo/bar"}, nil, nil),
			},
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.addControllerToStatus(
					f.siteStatus("mysite", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
					"bar",
					"foo",
				),
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
				f.skupperNetworkStatus("test", fakeNetworkStatus.info()),
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
				f.addNetworkStatus(f.siteStatus("mysite", "test", skupperv2alpha1.StatusReady, "OK"), fakeNetworkStatus),
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
				f.routerConfig("skupper-router", "test").tcpListener("mylistener", "1024", "", ""),
			},
			expectedServices: []*corev1.Service{
				f.service("mysvc", "test", f.routerSelector(true), f.servicePort("mylistener", 8080, 1024)),
			},
		},
		{
			name: "expose pods by name",
			k8sObjects: []runtime.Object{
				f.skupperNetworkStatus("test", f.networkStatusInfo("mysite", "test", nil, map[string]string{"mysvc": "mysvc", "mysvc.mypod-1": "10.1.1.10", "mysvc.mypod-2": "10.1.1.11"}).info()),
				f.pod("mypod-1", "test", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
				f.pod("mypod-2", "test", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.11", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
				f.connectorWithExposePodsByName("myconnector", "test", "mysvc", "app=foo", 8080),
				f.listenerWithExposePodsByName("mylistener", "test", "mysvc", "mysvc", 8080),
			},
			waitFunctions: []WaitFunction{
				isConnectorStatusConditionTrue("myconnector", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysite", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.routerDeployment("skupper-router", "test"),
			},
			expectedRouterConfig: []*RouterConfig{
				f.routerConfig(
					"skupper-router",
					"test",
				).tcpListener(
					"mylistener", "1024", "mysvc", "",
				).tcpListener(
					"mylistener@mypod-1", "*", "mysvc.mypod-1", "",
				).tcpListener(
					"mylistener@mypod-2", "*", "mysvc.mypod-2", "",
				).tcpConnector(
					"myconnector@10.1.1.10", "10.1.1.10", "8080", "mysvc", "",
				).tcpConnector(
					"myconnector@10.1.1.11", "10.1.1.11", "8080", "mysvc", "",
				).tcpConnector(
					"myconnector@mypod-1", "10.1.1.10", "8080", "mysvc.mypod-1", "",
				).tcpConnector(
					"myconnector@mypod-2", "10.1.1.11", "8080", "mysvc.mypod-2", "",
				),
			},
			expectedServices: []*corev1.Service{
				f.service("mysvc", "test", f.routerSelector(true), f.servicePort("mylistener", 8080, 1024)),
				f.service("mypod-1", "test", f.routerSelector(true), f.servicePortU("mylistener", 8080)),
				f.service("mypod-2", "test", f.routerSelector(true), f.servicePortU("mylistener", 8080)),
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
				f.routerConfig("skupper-router", "test").tcpConnector("myconnector@mysvc", "mysvc", "8080", "", ""),
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
				f.routerConfig("skupper-router", "test").tcpConnector("myconnector@10.1.1.10", "10.1.1.10", "8080", "", ""),
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
				f.routerConfig("skupper-router", "test").tcpConnector("myconnector@10.1.1.10", "10.1.1.10", "8080", "", ""),
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
		{
			name: "multiple site recovery",
			env: map[string]string{
				"NAMESPACE":       "foo",
				"CONTROLLER_NAME": "bar",
			},
			k8sObjects: []runtime.Object{
				f.routerConfig("skupper-router", "test").asConfigMapWithOwner("siteB", "49b03ad4-d414-42be-bbb5-b32d7d4ca503"),
			},
			skupperObjects: []runtime.Object{
				f.site("siteA", "test", "", false, false),
				f.addUID(f.site("siteB", "test", "", false, false), "49b03ad4-d414-42be-bbb5-b32d7d4ca503"),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("siteA", "test", skupperv2alpha1.StatusError, "An active site already exists in the namespace (siteB)"),
				f.siteStatus("siteB", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
		},
		{
			name: "site with default size",
			k8sObjects: []runtime.Object{
				f.siteSizing("default", "", map[string]string{
					"router-cpu-request":    "0.6",
					"router-memory-request": "500M",
				}),
			},
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
			},
			expectedSiteStatuses: []*skupperv2alpha1.Site{
				f.siteStatus("mysite", "test", skupperv2alpha1.StatusPending, "Not Running", f.condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
			},
			expectedDynamicResources: map[schema.GroupVersionResource]*unstructured.Unstructured{
				resource.DeploymentResource(): f.resources().routerMemoryRequest("500M").routerCpuRequest("600m").deployment("skupper-router", "test"),
			},
		},
		{
			name: "labelling",
			k8sObjects: []runtime.Object{
				f.configmap("labels", "test", nil, map[string]string{"skupper.io/label-template": "true", "acme.com/foo": "bar"}, nil),
			},
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
				f.routerConfig("skupper-router", "test").tcpListener("mylistener", "1024", "", ""),
			},
			expectedServices: []*corev1.Service{
				f.serviceWithMetadata(f.service("mysvc", "test", f.routerSelector(true), f.servicePort("mylistener", 8080, 1024)), map[string]string{"acme.com/foo": "bar"}, nil),
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
				controller.eventProcessor.TestProcess()
			}
			for _, wf := range tt.waitFunctions {
				for !wf(t, clients) {
					controller.eventProcessor.TestProcess()
				}
			}
			for _, expected := range tt.expectedSiteStatuses {
				actual, err := clients.GetSkupperClient().SkupperV2alpha1().Sites(expected.Namespace).Get(context.Background(), expected.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				verifyStatus(t, expected.Status.Status, actual.Status.Status)
				if expected.Status.Network != nil {
					assert.DeepEqual(t, mapByName(expected.Status.Network), mapByName(actual.Status.Network))
				}
				if expected.Status.Controller != nil {
					assert.DeepEqual(t, expected.Status.Controller, actual.Status.Controller)
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
				assert.Assert(t, match(expected.UnstructuredContent(), actual.UnstructuredContent()))
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
				assert.Equal(t, len(expected.Spec.Ports), len(actual.Spec.Ports))
				for i, port := range expected.Spec.Ports {
					// in some cases it is not possible to know the order in which ports on the router are assigned, use * as a wildcard in such cases
					if port.TargetPort.String() == "*" {
						expected.Spec.Ports[i].TargetPort = actual.Spec.Ports[i].TargetPort
					}
				}
				assert.DeepEqual(t, expected.Spec.Ports, actual.Spec.Ports)
				for k, v := range expected.ObjectMeta.Labels {
					assert.Assert(t, actual.ObjectMeta.Labels != nil)
					assert.Equal(t, actual.ObjectMeta.Labels[k], v)
				}
				for k, v := range expected.ObjectMeta.Annotations {
					assert.Assert(t, actual.ObjectMeta.Annotations != nil)
					assert.Equal(t, actual.ObjectMeta.Annotations[k], v)
				}
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

func TestUpdate(t *testing.T) {
	testTable := []struct {
		name           string
		args           []string
		env            map[string]string
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		functions      []WaitFunction
		postFunctions  []ControllerFunction
	}{
		{
			name: "change listener host",
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			functions: []WaitFunction{
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("mysvc", "test").check,
				updateListener("mylistener", "test", "adifferentsvc", 8080),
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("adifferentsvc", "test").check,
				negativeServiceCheck("mysvc", "test"),
			},
		}, {
			name: "exposePodsByName handles pod delete",
			k8sObjects: []runtime.Object{
				f.skupperNetworkStatus("test", f.networkStatusInfo("mysite", "test", map[string]string{"mysvc": "mysvc", "mysvc.mypod-1": "mysvc@mypod-1", "mysvc.mypod-2": "mysvc@mypod-2"}, map[string]string{"mysvc": "mysvc", "mysvc.mypod-1": "10.1.1.10", "mysvc.mypod-2": "10.1.1.11"}).info()),
				f.pod("mypod-1", "test", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
				f.pod("mypod-2", "test", map[string]string{"app": "foo"}, nil, f.podStatus("10.1.1.11", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
				f.connectorWithExposePodsByName("myconnector", "test", "mysvc", "app=foo", 8080),
				f.listenerWithExposePodsByName("mylistener", "test", "mysvc", "mysvc", 8080),
			},
			functions: []WaitFunction{
				isConnectorStatusConditionTrue("myconnector", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("mysvc", "test").check,
				serviceCheck("mypod-1", "test").check,
				deleteTargetPod("mypod-1", "test"),
				serviceCheck("mypod-1", "test").checkAbsent,
			},
		}, {
			name: "unreferenced attached connector",
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
				f.attachedConnectorBinding("myconnector", "test", "test2"),
				f.attachedConnector("myconnector", "test2", "test", "app=foo", 8080),
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			k8sObjects: []runtime.Object{
				f.pod("foo", "test2", map[string]string{"app": "foo"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			functions: []WaitFunction{
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("mysvc", "test").check,
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorStatusConditionTrue("myconnector", "test2", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				updateAttachedConnectorSiteNamespace("myconnector", "test2", "test3", skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse),
			},
			postFunctions: []ControllerFunction{
				podWatchers(1, 1),
			},
		}, {
			name: "deleted attached connector",
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
				f.attachedConnectorBinding("myconnector", "test", "test2"),
				f.attachedConnector("myconnector", "test2", "test", "app=foo", 8080),
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			k8sObjects: []runtime.Object{
				f.pod("foo", "test2", map[string]string{"app": "foo"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			functions: []WaitFunction{
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("mysvc", "test").check,
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorStatusConditionTrue("myconnector", "test2", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				deleteAttachedConnector("myconnector", "test2"),
			},
			postFunctions: []ControllerFunction{
				podWatchers(1, 1),
			},
		}, {
			name: "unreferenced attached connector binding",
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
				f.attachedConnectorBinding("myconnector", "test", "test2"),
				f.attachedConnector("myconnector", "test2", "test", "app=foo", 8080),
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			k8sObjects: []runtime.Object{
				f.pod("foo", "test2", map[string]string{"app": "foo"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			functions: []WaitFunction{
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("mysvc", "test").check,
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorStatusConditionTrue("myconnector", "test2", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorBindingStatusCondition("myconnector", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue),
				updateAttachedConnectorBindingConnectorNamespace("myconnector", "test", "test3", skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse),
				isAttachedConnectorBindingStatusCondition("myconnector", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse),
			},
			postFunctions: []ControllerFunction{
				podWatchers(1, 1),
			},
		}, {
			name: "deleted attached connector binding",
			skupperObjects: []runtime.Object{
				f.site("mysite", "test", "", false, false),
				f.attachedConnectorBinding("myconnector", "test", "test2"),
				f.attachedConnector("myconnector", "test2", "test", "app=foo", 8080),
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			k8sObjects: []runtime.Object{
				f.pod("foo", "test2", map[string]string{"app": "foo"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			functions: []WaitFunction{
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("mysvc", "test").check,
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorStatusConditionTrue("myconnector", "test2", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorBindingStatusCondition("myconnector", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue),
				deleteAttachedConnectorBinding("myconnector", "test"),
				isAttachedConnectorStatusCondition("myconnector", "test2", skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionFalse),
			},
			postFunctions: []ControllerFunction{
				podWatchers(1, 1),
			},
		}, {
			name: "site deleted",
			skupperObjects: []runtime.Object{
				f.addUID(f.site("mysite", "test", "", false, false), "49b03ad4-d414-42be-bbb5-b32d7d4ca503"),
				f.attachedConnectorBinding("myconnector", "test", "test2"),
				f.attachedConnector("myconnector", "test2", "test", "app=foo", 8080),
				f.listener("mylistener", "test", "mysvc", 8080),
			},
			k8sObjects: []runtime.Object{
				f.pod("foo", "test2", map[string]string{"app": "foo"}, nil,
					f.podStatus("10.1.1.10", corev1.PodRunning, f.podCondition(corev1.PodReady, corev1.ConditionTrue))),
			},
			functions: []WaitFunction{
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				serviceCheck("mysvc", "test").check,
				isListenerStatusConditionTrue("mylistener", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorStatusConditionTrue("myconnector", "test2", skupperv2alpha1.CONDITION_TYPE_CONFIGURED),
				isAttachedConnectorBindingStatusCondition("myconnector", "test", skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue),
				deleteSite("mysite", "test"),
			},
			postFunctions: []ControllerFunction{
				podWatchers(1, 1),
			},
		},
	}
	eventProcessorResyncShort := []watchers.EventProcessorCustomizer{
		func(e *watchers.EventProcessor) {
			e.SetResyncShort(time.Second)
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
			controller, err := NewController(clients, config, eventProcessorResyncShort...)
			assert.Assert(t, err)
			stopCh := make(chan struct{})
			err = controller.init(stopCh)
			assert.Assert(t, err)
			for i := 0; i < len(tt.k8sObjects)+len(tt.skupperObjects); i++ {
				controller.eventProcessor.TestProcess()
			}

			for _, f := range tt.functions {
				for !f(t, clients) {
					controller.eventProcessor.TestProcess()
				}
			}
			for _, f := range tt.postFunctions {
				for !f(t, controller) {
					controller.eventProcessor.TestProcess()
				}
			}
		})
	}
}

func deleteAttachedConnector(name string, namespace string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		err := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectors(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		assert.Assert(t, err)
		return true
	}
}

func deleteAttachedConnectorBinding(name string, namespace string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		err := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectorBindings(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		assert.Assert(t, err)
		return true
	}
}

func verifyStatus(t *testing.T, expected skupperv2alpha1.Status, actual skupperv2alpha1.Status) {
	t.Helper()
	assert.Equal(t, expected.StatusType, actual.StatusType, actual.Message)
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

func mapByName(records []skupperv2alpha1.SiteRecord) map[string]skupperv2alpha1.SiteRecord {
	index := map[string]skupperv2alpha1.SiteRecord{}
	for _, record := range records {
		index[record.Name] = record
	}
	return index
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

func (*factory) configMapWithOwner(name string, namespace string, owners ...metav1.OwnerReference) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: owners,
		},
	}
}

func (*factory) ownerRef(name string, uid string) metav1.OwnerReference {
	return metav1.OwnerReference{
		Name: name,
		UID:  types.UID(uid),
	}
}

func (*factory) addUID(site *skupperv2alpha1.Site, uid string) *skupperv2alpha1.Site {
	site.ObjectMeta.UID = types.UID(uid)
	return site
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

func (*factory) listenerWithExposePodsByName(name string, namespace string, key string, host string, port int) *skupperv2alpha1.Listener {
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
			Host:             host,
			Port:             port,
			ExposePodsByName: true,
			RoutingKey:       key,
		},
	}
}

func (*factory) connectorWithExposePodsByName(name string, namespace string, key string, selector string, port int) *skupperv2alpha1.Connector {
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
			Selector:         selector,
			Port:             port,
			ExposePodsByName: true,
			RoutingKey:       key,
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

func (*factory) addControllerToStatus(site *skupperv2alpha1.Site, name string, namespace string) *skupperv2alpha1.Site {
	site.Status.Controller = &skupperv2alpha1.Controller{
		Name:      name,
		Namespace: namespace,
		Version:   version.Version,
	}
	return site
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

type ExpectedResources struct {
	routerRequests  map[string]string
	routerLimits    map[string]string
	adaptorRequests map[string]string
	adaptorLimits   map[string]string
}

func (f *factory) resources() *ExpectedResources {
	return &ExpectedResources{}
}

func (r *ExpectedResources) routerMemoryLimit(value string) *ExpectedResources {
	if r.routerLimits == nil {
		r.routerLimits = map[string]string{}
	}
	r.routerLimits["memory"] = value
	return r
}

func (r *ExpectedResources) routerCpuLimit(value string) *ExpectedResources {
	if r.routerLimits == nil {
		r.routerLimits = map[string]string{}
	}
	r.routerLimits["cpu"] = value
	return r
}

func (r *ExpectedResources) routerMemoryRequest(value string) *ExpectedResources {
	if r.routerRequests == nil {
		r.routerRequests = map[string]string{}
	}
	r.routerRequests["memory"] = value
	return r
}

func (r *ExpectedResources) routerCpuRequest(value string) *ExpectedResources {
	if r.routerRequests == nil {
		r.routerRequests = map[string]string{}
	}
	r.routerRequests["cpu"] = value
	return r
}

func (r *ExpectedResources) adaptorMemoryLimit(value string) *ExpectedResources {
	if r.adaptorLimits == nil {
		r.adaptorLimits = map[string]string{}
	}
	r.adaptorLimits["memory"] = value
	return r
}

func (r *ExpectedResources) adaptorCpuLimit(value string) *ExpectedResources {
	if r.adaptorLimits == nil {
		r.adaptorLimits = map[string]string{}
	}
	r.adaptorLimits["cpu"] = value
	return r
}

func (r *ExpectedResources) adaptorMemoryRequest(value string) *ExpectedResources {
	if r.adaptorRequests == nil {
		r.adaptorRequests = map[string]string{}
	}
	r.adaptorRequests["memory"] = value
	return r
}

func (r *ExpectedResources) adaptorCpuRequest(value string) *ExpectedResources {
	if r.adaptorRequests == nil {
		r.adaptorRequests = map[string]string{}
	}
	r.adaptorRequests["cpu"] = value
	return r
}

func (r *ExpectedResources) routerResources() map[string]interface{} {
	m := map[string]interface{}{}
	if len(r.routerLimits) > 0 {
		m["limits"] = r.routerLimits
	}
	if len(r.routerRequests) > 0 {
		m["requests"] = r.routerRequests
	}
	return m
}

func (r *ExpectedResources) adaptorResources() map[string]interface{} {
	m := map[string]interface{}{}
	if len(r.adaptorLimits) > 0 {
		m["limits"] = r.adaptorLimits
	}
	if len(r.adaptorRequests) > 0 {
		m["requests"] = r.adaptorRequests
	}
	return m
}

func (r *ExpectedResources) deployment(name string, namespace string) *unstructured.Unstructured {
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
							"name":      "router",
							"resources": r.routerResources(),
						},
						{
							"name":      "kube-adaptor",
							"resources": r.adaptorResources(),
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

func (*factory) serviceWithMetadata(svc *corev1.Service, labels map[string]string, annotations map[string]string) *corev1.Service {
	svc.ObjectMeta.Labels = labels
	svc.ObjectMeta.Annotations = annotations
	return svc
}

func (*factory) servicePort(name string, port int32, targetPort int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.IntOrString{IntVal: int32(targetPort)},
		Protocol:   corev1.Protocol("TCP"),
	}
}

// where the target port cannot be determined in advance, use * as wildcard
func (*factory) servicePortU(name string, port int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.FromString("*"),
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

func (*factory) addNetworkStatus(site *skupperv2alpha1.Site, status *NetworkStatus) *skupperv2alpha1.Site {
	//add equivalent data to that generated in networkStatusInfo
	site.Status.Network = network.ExtractSiteRecords(*status.info())
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

func (*factory) siteSizing(name string, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"skupper.io/site-sizing": "default",
			},
			Annotations: map[string]string{
				"skupper.io/default-site-sizing": "",
			},
		},
		Data: data,
	}
}

func (*factory) configmap(name string, namespace string, data map[string]string, labels map[string]string, annotations map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: data,
	}
}

func (f *factory) fakeNetworkStatusInfo(namespace string) *NetworkStatus {
	return f.networkStatusInfo(
		"east", namespace, map[string]string{"mysvc": "mysvc"}, map[string]string{"mysvc": "foo"},
	).site(
		"west", "other", nil, nil,
	).link(
		"west", "east",
	)
}

type NetworkStatus struct {
	sites map[string]*network.SiteStatusInfo
}

func (n *NetworkStatus) info() *network.NetworkStatusInfo {
	info := &network.NetworkStatusInfo{}
	for _, site := range n.sites {
		info.SiteStatus = append(info.SiteStatus, *site)
	}
	return info
}

func (n *NetworkStatus) site(name string, namespace string, listeners map[string]string, connectors map[string]string) *NetworkStatus {
	router := network.RouterStatusInfo{
		Router: network.RouterInfo{
			Name:      name + "-skupper-router-" + rand.String(10) + "-" + rand.String(5),
			Namespace: namespace,
		},
		AccessPoints: []network.RouterAccessInfo{
			{
				Identity: uuid.NewString(),
			},
		},
	}
	for address, name := range listeners {
		router.Listeners = append(router.Listeners, network.ListenerInfo{
			Name:    name,
			Address: address,
		})
	}
	for address, host := range connectors {
		router.Connectors = append(router.Connectors, network.ConnectorInfo{
			Address:  address,
			DestHost: host,
		})

	}

	n.sites[name] = &network.SiteStatusInfo{
		Site: network.SiteInfo{
			Identity:  uuid.NewString(),
			Name:      name,
			Namespace: namespace,
		},
		RouterStatus: []network.RouterStatusInfo{
			router,
		},
	}
	return n
}

func (n *NetworkStatus) link(from string, to string) *NetworkStatus {
	siteA, ok1 := n.sites[from]
	siteB, ok2 := n.sites[to]
	if ok1 && ok2 && len(siteA.RouterStatus[0].AccessPoints) > 0 {
		link := network.LinkInfo{
			Name: fmt.Sprintf("link-%d", len(siteA.RouterStatus[0].Links)+1),
			Peer: siteB.RouterStatus[0].AccessPoints[0].Identity,
		}
		siteA.RouterStatus[0].Links = append(siteA.RouterStatus[0].Links, link)
	}
	return n
}

func (*factory) networkStatusInfo(name string, namespace string, listeners map[string]string, connectors map[string]string) *NetworkStatus {
	n := &NetworkStatus{
		sites: map[string]*network.SiteStatusInfo{},
	}
	return n.site(name, namespace, listeners, connectors)
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
			a := expected.MapIndex(key)
			b := actual.MapIndex(key)
			if !a.IsValid() && !b.IsValid() {
				return nil
			}
			if a.IsValid() != b.IsValid() {
				return fmt.Errorf("%s does not match: expected %v, got %v", key, a, b)
			}
			if err := match(expected.MapIndex(key).Interface(), actual.MapIndex(key).Interface()); err != nil {
				return fmt.Errorf("Value for %s does not match: %s", key.String(), err)
			}
		}
		return nil
	} else if expected.Kind() == reflect.Slice || expected.Kind() == reflect.Array {
		if expected.Len() != actual.Len() {
			return fmt.Errorf("Different number of items, expected %s, got %s", expected.String(), actual.String())
		}
		for i := 0; i < expected.Len(); i++ {
			if err := match(expected.Index(i).Interface(), actual.Index(i).Interface()); err != nil {
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

func (rc *RouterConfig) tcpListener(name string, port string, address string, sslProfile string) *RouterConfig {
	rc.config.Bridges.TcpListeners[name] = qdr.TcpEndpoint{
		Name:       name,
		Port:       port,
		Address:    address,
		SslProfile: sslProfile,
	}
	return rc
}

func (rc *RouterConfig) tcpConnector(name string, host string, port string, address string, sslProfile string) *RouterConfig {
	rc.config.Bridges.TcpConnectors[name] = qdr.TcpEndpoint{
		Name:       name,
		Host:       host,
		Port:       port,
		Address:    address,
		SslProfile: sslProfile,
	}
	return rc
}

func (rc *RouterConfig) asConfigMapWithOwner(name string, uid string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rc.name,
			Namespace: rc.namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name: name,
					UID:  types.UID(uid),
				},
			},
		},
	}
	rc.config.WriteToConfigMap(cm)
	return cm
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
		if expected.Port == "*" {
			// in some cases it is not possible to deterministically infer the order in which ports will be assigned
			expected.Port = actual.Port
		}
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
		site, err := clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return meta.IsStatusConditionTrue(site.Status.Conditions, condition)
	}
}

func isSiteNetworkStatusSet(name string, namespace string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		site, err := clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return site.Status.Network != nil
	}
}

func isConnectorStatusConditionTrue(name string, namespace string, condition string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		connector, err := clients.GetSkupperClient().SkupperV2alpha1().Connectors(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return meta.IsStatusConditionTrue(connector.Status.Conditions, condition)
	}
}

func isConditionUpToDate(conditions []metav1.Condition, conditionType string, generation int64) bool {
	cond := meta.FindStatusCondition(conditions, conditionType)
	return cond != nil && cond.ObservedGeneration == generation
}

func isListenerStatusConditionTrue(name string, namespace string, condition string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		listener, err := clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return isConditionUpToDate(listener.Status.Conditions, condition, listener.ObjectMeta.Generation) && meta.IsStatusConditionTrue(listener.Status.Conditions, condition)
	}
}

func updateAttachedConnectorSiteNamespace(name string, namespace string, siteNamespace string, condition string, conditionStatus metav1.ConditionStatus) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		cli := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectors(namespace)
		connector, err := cli.Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		if connector.Spec.SiteNamespace != siteNamespace {
			t.Logf("updating siteNamespace")
			connector.Spec.SiteNamespace = siteNamespace
			_, err = cli.Update(context.Background(), connector, metav1.UpdateOptions{})
			assert.Assert(t, err)
			return false
		}
		return isConditionUpToDate(connector.Status.Conditions, condition, connector.ObjectMeta.Generation) && meta.IsStatusConditionPresentAndEqual(connector.Status.Conditions, condition, conditionStatus)
	}
}

func updateAttachedConnectorBindingConnectorNamespace(name string, namespace string, connectorNamespace string, condition string, status metav1.ConditionStatus) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		cli := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectorBindings(namespace)
		binding, err := cli.Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		if binding.Spec.ConnectorNamespace != connectorNamespace {
			t.Logf("updating connectorNamespace")
			binding.Spec.ConnectorNamespace = connectorNamespace
			_, err = cli.Update(context.Background(), binding, metav1.UpdateOptions{})
			assert.Assert(t, err)
			return false
		}
		return isConditionUpToDate(binding.Status.Conditions, condition, binding.ObjectMeta.Generation) && meta.IsStatusConditionPresentAndEqual(binding.Status.Conditions, condition, status)
	}
}

func isAttachedConnectorStatusConditionTrue(name string, namespace string, condition string) WaitFunction {
	return isAttachedConnectorStatusCondition(name, namespace, condition, metav1.ConditionTrue)
}

func isAttachedConnectorStatusCondition(name string, namespace string, condition string, status metav1.ConditionStatus) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		connector, err := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectors(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return meta.IsStatusConditionPresentAndEqual(connector.Status.Conditions, condition, status)
	}
}

func isAttachedConnectorBindingStatusCondition(name string, namespace string, condition string, status metav1.ConditionStatus) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		binding, err := clients.GetSkupperClient().SkupperV2alpha1().AttachedConnectorBindings(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, err)
		return meta.IsStatusConditionPresentAndEqual(binding.Status.Conditions, condition, status)
	}
}

func podWatchers(expectedRunning int, expectedStopped int) ControllerFunction {
	return func(t *testing.T, controller *Controller) bool {
		for {
			var running int
			var stopped int
			for _, w := range controller.eventProcessor.GetWatchers() {
				if rw, ok := w.(*watchers.ResourceWatcher[*corev1.Pod]); ok {
					if rw.IsStopped() {
						stopped++
					} else {
						running++
					}
				}
			}
			if expectedRunning == running && expectedStopped == stopped {
				return true
			}
			t.Logf("PodWatchers count do not match - Expected Running %d/%d - Expected Stopped %d/%d", running, expectedRunning, stopped, expectedStopped)
			return false
		}
	}
}

func deleteSite(name string, namespace string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		site, err := clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err == nil && site != nil {
			_ = clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
			return false
		}
		return true
	}
}

type ServiceCheck struct {
	name         string
	namespace    string
	ports_       []corev1.ServicePort
	selector_    map[string]string
	labels_      map[string]string
	annotations_ map[string]string
}

func serviceCheck(name string, namespace string) *ServiceCheck {
	return &ServiceCheck{
		name:      name,
		namespace: namespace,
	}
}

func (s *ServiceCheck) check(t *testing.T, clients internalclient.Clients) bool {
	actual, err := clients.GetKubeClient().CoreV1().Services(s.namespace).Get(context.Background(), s.name, metav1.GetOptions{})
	assert.Assert(t, err)
	if s.selector_ != nil {
		assert.DeepEqual(t, s.selector_, actual.Spec.Selector)
	}
	if s.ports_ != nil {
		assert.Equal(t, len(s.ports_), len(actual.Spec.Ports))
		for i, port := range s.ports_ {
			// in some cases it is not possible to know the order in which ports on the router are assigned, use * as a wildcard in such cases
			if port.TargetPort.String() == "*" {
				s.ports_[i].TargetPort = actual.Spec.Ports[i].TargetPort
			}
		}
		assert.DeepEqual(t, s.ports_, actual.Spec.Ports)
	}
	for k, v := range s.labels_ {
		assert.Assert(t, actual.ObjectMeta.Labels != nil)
		assert.Equal(t, actual.ObjectMeta.Labels[k], v)
	}
	for k, v := range s.annotations_ {
		assert.Assert(t, actual.ObjectMeta.Annotations != nil)
		assert.Equal(t, actual.ObjectMeta.Annotations[k], v)
	}
	return true
}

func (s *ServiceCheck) checkAbsent(t *testing.T, clients internalclient.Clients) bool {
	_, err := clients.GetKubeClient().CoreV1().Services(s.namespace).Get(context.Background(), s.name, metav1.GetOptions{})
	if err == nil {
		return false
	}
	if errors.IsNotFound(err) {
		return true
	}
	assert.Assert(t, err)
	return false
}

func updateListener(name string, namespace string, host string, port int) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		ctxt := context.Background()
		current, err := clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctxt, name, metav1.GetOptions{})
		assert.Assert(t, err)
		current.Spec.Host = host
		current.Spec.Port = port
		current.ObjectMeta.Generation++
		_, err = clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Update(ctxt, current, metav1.UpdateOptions{})
		assert.Assert(t, err)
		return true
	}
}

func negativeServiceCheck(name string, namespace string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		_, err := clients.GetKubeClient().CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.Assert(t, errors.IsNotFound(err))
		return true
	}
}

func deleteTargetPod(name string, namespace string) WaitFunction {
	return func(t *testing.T, clients internalclient.Clients) bool {
		ctxt := context.Background()
		err := clients.GetKubeClient().CoreV1().Pods(namespace).Delete(ctxt, name, metav1.DeleteOptions{})
		assert.Assert(t, err)
		cm, err := clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctxt, "skupper-network-status", metav1.GetOptions{})
		assert.Assert(t, err)
		cm = cm.DeepCopy()
		var status network.NetworkStatusInfo
		assert.Assert(t, json.Unmarshal([]byte(cm.Data["NetworkStatus"]), &status))
		suffix := "." + name
		for sIdx := range status.SiteStatus {
			for rIdx, rs := range status.SiteStatus[sIdx].RouterStatus {
				connectors := rs.Connectors[:0]
				for _, c := range rs.Connectors {
					if !strings.HasSuffix(c.Address, suffix) {
						connectors = append(connectors, c)
					}
				}
				status.SiteStatus[sIdx].RouterStatus[rIdx].Connectors = connectors
			}
		}
		sb, err := json.Marshal(status)
		assert.Assert(t, err)
		cm.Data["NetworkStatus"] = string(sb)
		_, err = clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Update(ctxt, cm, metav1.UpdateOptions{})
		assert.Assert(t, err)
		return true
	}
}
