//go:build integration

package kubecontrollertest

import (
	"context"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/fixtures"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

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
}
