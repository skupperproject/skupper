//go:build integration

package kubecontrollertest

import (
	"context"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/fixtures"
	"github.com/skupperproject/skupper/internal/qdr"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type RouterConfigReadyFn func(config *qdr.RouterConfig) bool

func TestSimpleSite(t *testing.T) {
	tc := setup(t)
	namespace := "simple-site"
	tc.createNamespace(namespace)

	ctx := context.Background()
	_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Create(ctx, fixtures.Site("mysite", namespace), metav1.CreateOptions{})
	assert.NilError(t, err)

	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {
		actual, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(ctx, "mysite", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		configured := meta.FindStatusCondition(actual.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_CONFIGURED)
		if configured == nil || configured.Status != metav1.ConditionTrue {
			return false, nil
		}
		return true, nil
	})

	actualSite, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(ctx, "mysite", metav1.GetOptions{})
	assert.NilError(t, err)
	verifyStatus(t, fixtures.Status(skupperv2alpha1.StatusPending, "Not Running",
		fixtures.Condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
		actualSite.Status.Status)

	deployment, err := tc.clients.GetKubeClient().AppsV1().Deployments(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, deployment.Labels["skupper.io/component"], "router")
	assert.Equal(t, deployment.Labels["application"], "skupper-router")
	assert.Equal(t, len(deployment.Spec.Template.Spec.Containers), 2)

	t.Run("set network-id", func(t *testing.T) {
		actualSite.Spec.NetworkId = "my-van"
		actualSite, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Update(ctx, actualSite, metav1.UpdateOptions{})
		assert.NilError(t, err)
		waitForRouterConfigState(t, tc, namespace, func(config *qdr.RouterConfig) bool {
			return config.Network.NetworkId != ""
		})
	})

	t.Run("add routerAccess with dynamic port allocation and keys", func(t *testing.T) {
		ra := fixtures.RouterAccess("inter-van-ra", namespace)
		ra.Spec.AccessType = "local"
		ra.Spec.Roles = append(ra.Spec.Roles, skupperv2alpha1.RouterAccessRole{
			Name: "inter-network",
		})
		ra.Spec.RoutingKeys = append(ra.Spec.RoutingKeys, "key1", "key2")
		ra, err = tc.clients.GetSkupperClient().SkupperV2alpha1().RouterAccesses(namespace).Create(ctx, ra, metav1.CreateOptions{})
		assert.NilError(t, err)
	})

	t.Run("validate expected autoLinks present", func(t *testing.T) {
		expectedAutoLinks := []qdr.AutoLink{
			{
				Name:            "routerAccess/inter-van-ra-inter-network",
				ExternalAddress: "_xtopo/my-van",
				Direction:       "in",
				Connection:      "inter-van-ra-inter-network",
			},
			{
				Name:       "routerAccess/inter-van-ra/key1",
				Address:    "key1",
				Direction:  "in",
				Connection: "inter-van-ra-inter-network",
			},
			{
				Name:       "routerAccess/inter-van-ra/key2",
				Address:    "key2",
				Direction:  "in",
				Connection: "inter-van-ra-inter-network",
			},
		}
		waitForRouterConfigState(t, tc, namespace, func(config *qdr.RouterConfig) bool {
			if len(config.AutoLinks) != len(expectedAutoLinks) {
				return false
			}
			for name, autoLink := range config.AutoLinks {
				var found bool
				for _, wantedAutoLink := range expectedAutoLinks {
					if wantedAutoLink.Name == name && autoLink == wantedAutoLink {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		})
	})

	t.Run("create an inter-van link with exposed routingKeys", func(t *testing.T) {
		link := fixtures.Link("link-van-1", namespace)
		link.Spec.Endpoints = []skupperv2alpha1.Endpoint{
			{
				Name:  "inter-network",
				Group: "skupper-router",
				Host:  "link-host",
				Port:  "35671",
			},
		}
		link.Spec.RoutingKeys = []string{"key1", "key2"}
		_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Links(namespace).Create(ctx, link, metav1.CreateOptions{})
		assert.NilError(t, err)
	})

	t.Run("validate inter-network connector and autoLinks created", func(t *testing.T) {
		expectedAutoLinks := []qdr.AutoLink{
			{
				Name:            "link/link-van-1",
				ExternalAddress: "_xtopo/my-van",
				Direction:       "in",
				Connection:      "link-van-1",
			},
			{
				Name:       "link/link-van-1/key1",
				Address:    "key1",
				Direction:  "in",
				Connection: "link-van-1",
			},
			{
				Name:       "link/link-van-1/key2",
				Address:    "key2",
				Direction:  "in",
				Connection: "link-van-1",
			},
		}
		waitForRouterConfigState(t, tc, namespace, func(config *qdr.RouterConfig) bool {
			connector, ok := config.Connectors["link-van-1"]
			if !ok {
				return false
			}
			assert.Equal(t, "inter-network", string(connector.Role))
			for _, wanted := range expectedAutoLinks {
				got, ok := config.AutoLinks[wanted.Name]
				if !ok {
					return false
				}
				assert.Equal(t, wanted, got)
			}
			return true
		})
	})

	t.Run("remove networkId", func(t *testing.T) {
		actualSite, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(ctx, actualSite.Name, metav1.GetOptions{})
		assert.NilError(t, err)
		actualSite.Spec.NetworkId = ""
		actualSite, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Update(ctx, actualSite, metav1.UpdateOptions{})
		assert.NilError(t, err)
		waitForRouterConfigState(t, tc, namespace, func(config *qdr.RouterConfig) bool {
			return config.Network.NetworkId == ""
		})
	})

	t.Run("ensure topology address autoLinks removed", func(t *testing.T) {
		waitForRouterConfigState(t, tc, namespace, func(config *qdr.RouterConfig) bool {
			return len(config.AutoLinks) == 4
		})
	})

	t.Run("remove router access keys", func(t *testing.T) {
		ra, err := tc.clients.GetSkupperClient().SkupperV2alpha1().RouterAccesses(namespace).Get(ctx, "inter-van-ra", metav1.GetOptions{})
		assert.NilError(t, err)
		ra.Spec.RoutingKeys = nil
		_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().RouterAccesses(namespace).Update(ctx, ra, metav1.UpdateOptions{})
		assert.NilError(t, err)
	})

	t.Run("remove inter-van link keys", func(t *testing.T) {
		link, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Links(namespace).Get(ctx, "link-van-1", metav1.GetOptions{})
		assert.NilError(t, err)
		link.Spec.RoutingKeys = nil
		_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Links(namespace).Update(ctx, link, metav1.UpdateOptions{})
		assert.NilError(t, err)
	})

	t.Run("assert no autoLinks left but listener and connector are present", func(t *testing.T) {
		waitForRouterConfigState(t, tc, namespace, func(config *qdr.RouterConfig) bool {
			if len(config.AutoLinks) > 0 {
				return false
			}
			_, listenerFound := config.Listeners["inter-van-ra-inter-network"]
			_, connectorFound := config.Connectors["link-van-1"]
			assert.Assert(t, listenerFound)
			assert.Assert(t, connectorFound)
			return true
		})
	})
}

func waitForRouterConfigState(t *testing.T, tc *testContext, namespace string, state RouterConfigReadyFn) (routerConfig *qdr.RouterConfig) {
	waitFor(t, 10*time.Second, 100*time.Millisecond, func() (bool, error) {
		cm, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(context.Background(), "skupper-router", metav1.GetOptions{})
		assert.NilError(t, err)
		routerConfig, err = qdr.GetRouterConfigFromConfigMap(cm)
		assert.NilError(t, err)
		return state(routerConfig), nil
	})
	return
}
