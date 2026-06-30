//go:build integration

package kubecontrollertest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/fixtures"
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
	assert.Assert(t, strings.Contains(routerConfig.Data[types.TransportConfigFile], "listener/mylistener"))
}
