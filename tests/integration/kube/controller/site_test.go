//go:build integration

package kubecontrollertest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/fixtures"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

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

func TestSiteModeCRDValidation(t *testing.T) {
	tc := setup(t)
	ctx := context.Background()
	siteGVR := schema.GroupVersionResource{
		Group:    "skupper.io",
		Version:  "v2alpha1",
		Resource: "sites",
	}
	modePtr := func(m skupperv2alpha1.SiteMode) *skupperv2alpha1.SiteMode { return &m }

	t.Run("accepts all valid modes and rejects invalid mode", func(t *testing.T) {
		ns := "site-mode-enum"
		tc.createNamespace(ns)
		sites := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(ns)

		for _, mode := range []skupperv2alpha1.SiteMode{
			skupperv2alpha1.SiteModeDedicated,
			skupperv2alpha1.SiteModeDedicatedEdge,
			skupperv2alpha1.SiteModeTransit,
			skupperv2alpha1.SiteModeClusterShared,
			skupperv2alpha1.SiteModeClusterSharedSmall,
			skupperv2alpha1.SiteModeGuest,
		} {
			created, err := sites.Create(ctx, &skupperv2alpha1.Site{
				ObjectMeta: metav1.ObjectMeta{Name: "site-" + strings.ToLower(string(mode)), Namespace: ns},
				Spec:       skupperv2alpha1.SiteSpec{Mode: modePtr(mode)},
			}, metav1.CreateOptions{})
			assert.NilError(t, err)
			assert.Assert(t, created.Spec.Mode != nil)
			assert.Equal(t, *created.Spec.Mode, mode)
		}

		invalid := skupperv2alpha1.SiteMode("invalidMode")
		_, err := sites.Create(ctx, &skupperv2alpha1.Site{
			ObjectMeta: metav1.ObjectMeta{Name: "site-invalid", Namespace: ns},
			Spec:       skupperv2alpha1.SiteSpec{Mode: &invalid},
		}, metav1.CreateOptions{})
		assert.Assert(t, err != nil, "expected invalid spec.mode to be rejected")
	})

	t.Run("validates spec.edge and spec.mode consistency", func(t *testing.T) {
		ns := "site-mode-edge-consistency"
		tc.createNamespace(ns)
		sites := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(ns)

		_, err := sites.Create(ctx, &skupperv2alpha1.Site{
			ObjectMeta: metav1.ObjectMeta{Name: "legacy-edge", Namespace: ns},
			Spec:       skupperv2alpha1.SiteSpec{Edge: true},
		}, metav1.CreateOptions{})
		assert.NilError(t, err)

		_, err = sites.Create(ctx, &skupperv2alpha1.Site{
			ObjectMeta: metav1.ObjectMeta{Name: "matching-edge", Namespace: ns},
			Spec: skupperv2alpha1.SiteSpec{
				Edge: true,
				Mode: modePtr(skupperv2alpha1.SiteModeDedicatedEdge),
			},
		}, metav1.CreateOptions{})
		assert.NilError(t, err)

		for _, mode := range []skupperv2alpha1.SiteMode{
			skupperv2alpha1.SiteModeDedicated,
			skupperv2alpha1.SiteModeTransit,
			skupperv2alpha1.SiteModeClusterShared,
			skupperv2alpha1.SiteModeClusterSharedSmall,
			skupperv2alpha1.SiteModeGuest,
		} {
			_, err := sites.Create(ctx, &skupperv2alpha1.Site{
				ObjectMeta: metav1.ObjectMeta{Name: "conflict-" + strings.ToLower(string(mode)), Namespace: ns},
				Spec: skupperv2alpha1.SiteSpec{
					Edge: true,
					Mode: modePtr(mode),
				},
			}, metav1.CreateOptions{})
			assert.ErrorContains(t, err, "spec.edge cannot be true when spec.mode is set to a value other than 'dedicatedEdge'")
		}
	})

	t.Run("enforces immutability when mode is unset including omitted spec", func(t *testing.T) {
		ns := "site-mode-immutable-unset"
		tc.createNamespace(ns)
		dynSites := tc.clients.GetDynamicClient().Resource(siteGVR).Namespace(ns)
		sites := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(ns)

		// Create site with no spec field at all (upgrade edge case).
		_, err := dynSites.Create(ctx, &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "skupper.io/v2alpha1",
				"kind":       "Site",
				"metadata":   map[string]any{"name": "no-spec"},
			},
		}, metav1.CreateOptions{})
		assert.NilError(t, err)

		// Setting spec.mode from nil when old spec was omitted must be denied.
		current, err := sites.Get(ctx, "no-spec", metav1.GetOptions{})
		assert.NilError(t, err)
		current.Spec.Mode = modePtr(skupperv2alpha1.SiteModeDedicated)
		_, err = sites.Update(ctx, current, metav1.UpdateOptions{})
		assert.ErrorContains(t, err, "spec.mode is immutable")

		// Updating another spec field while keeping spec.mode nil must succeed.
		current, err = sites.Get(ctx, "no-spec", metav1.GetOptions{})
		assert.NilError(t, err)
		current.Spec.LinkAccess = "none"
		current, err = sites.Update(ctx, current, metav1.UpdateOptions{})
		assert.NilError(t, err)
		assert.Assert(t, current.Spec.Mode == nil)

		// Setting spec.mode from nil on an existing spec must still be denied.
		current.Spec.Mode = modePtr(skupperv2alpha1.SiteModeDedicated)
		_, err = sites.Update(ctx, current, metav1.UpdateOptions{})
		assert.ErrorContains(t, err, "spec.mode is immutable")
	})

	t.Run("enforces immutability when mode is set", func(t *testing.T) {
		ns := "site-mode-immutable-set"
		tc.createNamespace(ns)
		sites := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(ns)

		created, err := sites.Create(ctx, &skupperv2alpha1.Site{
			ObjectMeta: metav1.ObjectMeta{Name: "set-mode", Namespace: ns},
			Spec:       skupperv2alpha1.SiteSpec{Mode: modePtr(skupperv2alpha1.SiteModeDedicated)},
		}, metav1.CreateOptions{})
		assert.NilError(t, err)

		// Updating another field while keeping spec.mode unchanged must succeed.
		created.Spec.LinkAccess = "none"
		updated, err := sites.Update(ctx, created, metav1.UpdateOptions{})
		assert.NilError(t, err)

		// Changing spec.mode must be denied.
		updated.Spec.Mode = modePtr(skupperv2alpha1.SiteModeTransit)
		_, err = sites.Update(ctx, updated, metav1.UpdateOptions{})
		assert.ErrorContains(t, err, "spec.mode is immutable")

		// Clearing spec.mode must be denied.
		current, err := sites.Get(ctx, "set-mode", metav1.GetOptions{})
		assert.NilError(t, err)
		current.Spec.Mode = nil
		_, err = sites.Update(ctx, current, metav1.UpdateOptions{})
		assert.ErrorContains(t, err, "spec.mode is immutable")

		// Updating spec.edge to true on a dedicated site must be denied.
		current, err = sites.Get(ctx, "set-mode", metav1.GetOptions{})
		assert.NilError(t, err)
		current.Spec.Edge = true
		_, err = sites.Update(ctx, current, metav1.UpdateOptions{})
		assert.ErrorContains(t, err, "spec.edge cannot be true when spec.mode is set to a value other than 'dedicatedEdge'")
	})
}
