//go:build integration

package kubecontrollertest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/fixtures"
	kubeqdr "github.com/skupperproject/skupper/internal/kube/qdr"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func TestSiteWithListener(t *testing.T) {
	tc := setup(t)
	namespace := "site-with-listener"
	tc.createNamespace(namespace)

	ctx := context.Background()
	_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Create(ctx, fixtures.Site("mysvc", namespace), metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Create(ctx, listenerWithHostPort("mylistener", namespace, "mysvc", 8080), metav1.CreateOptions{})
	assert.NilError(t, err)

	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {
		l, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, "mylistener", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		configured := meta.FindStatusCondition(l.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_CONFIGURED)
		if configured == nil || configured.Status != metav1.ConditionTrue {
			return false, nil
		}
		_, err = tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "mysvc", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		return true, nil
	})

	actualSite, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Get(ctx, "mysvc", metav1.GetOptions{})
	assert.NilError(t, err)
	verifyStatus(t, fixtures.Status(skupperv2alpha1.StatusPending, "Not Running",
		fixtures.Condition(skupperv2alpha1.CONDITION_TYPE_CONFIGURED, metav1.ConditionTrue, "Ready", "OK")),
		actualSite.Status.Status)

	deployment, err := tc.clients.GetKubeClient().AppsV1().Deployments(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, deployment.Labels["skupper.io/component"], "router")

	svc, err := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "mysvc", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, svc.Spec.Selector, routerSelector())
	assert.Equal(t, len(svc.Spec.Ports), 1)
	assert.Equal(t, svc.Spec.Ports[0].Port, int32(8080))
	assert.Equal(t, svc.Labels["internal.skupper.io/listener"], "mylistener")

	routerConfig, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
	assert.NilError(t, err)
	routerConfigData, err := kubeqdr.GetRouterConfigData(routerConfig)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(routerConfigData, "listener/mylistener"))
}

func TestListenerWithoutSite(t *testing.T) {
	tc := setup(t)
	namespace := "listener-no-site"
	tc.createNamespace(namespace)

	ctx := context.Background()

	listener := listenerWithHostPort("test-listener", namespace, "test-service", 8080)
	_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Create(ctx, listener, metav1.CreateOptions{})
	assert.NilError(t, err)

	var actual *skupperv2alpha1.Listener
	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {
		var err error
		actual, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, "test-listener", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		if actual.Status.StatusType == skupperv2alpha1.StatusError {
			return true, nil
		}
		return false, nil
	})

	verifyStatus(t,
		fixtures.Status(skupperv2alpha1.StatusError, "No active site in namespace"),
		actual.Status.Status,
	)
}

func TestTwoListeners(t *testing.T) {

	// In this test we wait for the Services corresponding to both Listeners to show up,
	// and also separately wait for the names of both listeners to show up in the Router configmap.
	// (Don't assume that the Config is ready just because the Services are.)
	// If either of those don't happen within the timeout, the test fails.

	tc := setup(t)
	namespace := "multiple-listeners"
	tc.createNamespace(namespace)

	ctx := context.Background()

	// Create the Site.
	_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Create(ctx, fixtures.Site("mysite", namespace), metav1.CreateOptions{})
	assert.NilError(t, err)

	// Create two Listeners in the Site.
	_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Create(ctx,
		listenerWithHostPort("listener-a", namespace, "svc-a", 8080), metav1.CreateOptions{})
	assert.NilError(t, err)

	_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Create(ctx,
		listenerWithHostPort("listener-b", namespace, "svc-b", 9090), metav1.CreateOptions{})
	assert.NilError(t, err)

	// Wait until both Services exist for the two Listeners.
	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {
		_, errA := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "svc-a", metav1.GetOptions{})
		if done, err := retryOnNotFound(errA); !done {
			return false, err
		}
		_, errB := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "svc-b", metav1.GetOptions{})
		if done, err := retryOnNotFound(errB); !done {
			return false, err
		}
		return true, nil
	})

	// Wait until Router Config contains the names of both Listeners.
	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {
		routerConfig, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		cfg, err := kubeqdr.GetRouterConfigData(routerConfig)
		if err != nil {
			return false, err
		}
		return strings.Contains(cfg, "listener/listener-a") &&
			strings.Contains(cfg, "listener/listener-b"), nil
	})

	// Make sure the Services have the right Ports and Labels.
	svcA, err := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "svc-a", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(svcA.Spec.Ports), 1)
	assert.Equal(t, svcA.Spec.Ports[0].Port, int32(8080))
	assert.Equal(t, svcA.Labels["internal.skupper.io/listener"], "listener-a")

	svcB, err := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "svc-b", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(svcB.Spec.Ports), 1)
	assert.Equal(t, svcB.Spec.Ports[0].Port, int32(9090))
	assert.Equal(t, svcB.Labels["internal.skupper.io/listener"], "listener-b")
}
