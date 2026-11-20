package securedaccess

import (
	"context"
	"errors"
	"testing"

	"github.com/skupperproject/skupper/internal/kube/resource"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestScopeCleanupInventoriesSurvivingOutputs(t *testing.T) {
	h := newHarness(t, nil)
	sa := testAccess("local")
	sa.Spec.Issuer = "issuer"
	h.put(sa)
	h.settle("edge")
	// A legacy label spelling must not exclude a surviving TLSRoute from the
	// owner inventory, even though this Access no longer selects Gateway.
	legacy := desiredTLSRoute(sa, sa.Spec.Ports[0], "controller", "gateway.example")
	legacy.SetLabels(map[string]string{"internal.skupper.io/securedaccess": sa.Name})
	old := sa.DeepCopy()
	old.UID = "old-access-uid"
	// An extra non-controller owner does not block cleanup of the old output.
	legacy.SetOwnerReferences(append(ownerReferences(old), metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "backup", UID: "backup-uid"}))
	_, err := h.clients.Dynamic.Resource(resource.TlsRouteResource()).Namespace("test").Create(context.Background(), legacy, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = h.clients.Route.RouteV1().Routes("test").Create(context.Background(), desiredRouteForPort(old, old.Spec.Ports[0]), metav1.CreateOptions{})
	assert.NilError(t, err)
	proxy := legacy.DeepCopy()
	proxy.SetGroupVersionKind(httpProxyGVK)
	_, err = h.clients.Dynamic.Resource(httpProxyResource).Namespace("test").Create(context.Background(), proxy, metav1.CreateOptions{})
	assert.NilError(t, err)
	assert.NilError(t, h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Delete(context.Background(), "edge", metav1.DeleteOptions{}))
	h.observe()
	// A failed removal is retryable and does not erase discovery state.
	k := h.clients.Kube.(*k8sfake.Clientset)
	denied := errors.New("deletion denied")
	k.PrependReactor("delete", "services", func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, denied })
	assert.Assert(t, errors.Is(h.run("test/edge"), denied))
	k.ReactionChain = k.ReactionChain[1:]
	h = newHarness(t, h.clients)
	h.observe()
	h.m.config.EnabledAccessTypes = []string{"local"}
	// Domain cleanup from fresh caches; controller tests cover the actual
	// surviving-output informer route to these scope handlers.
	assert.NilError(t, h.run("test/edge"))
	assert.NilError(t, h.runCertificates())
	svcs, err := h.clients.Kube.CoreV1().Services("test").List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(svcs.Items), 0)
	routes, err := h.clients.Dynamic.Resource(resource.TlsRouteResource()).Namespace("test").List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(routes.Items), 0)
	proxies, err := h.clients.Dynamic.Resource(httpProxyResource).Namespace("test").List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(proxies.Items), 0)
	openShiftRoutes, err := h.clients.Route.RouteV1().Routes("test").List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(openShiftRoutes.Items), 0)
	_, err = h.clients.Skupper.SkupperV2alpha1().Certificates("test").Get(context.Background(), "edge", metav1.GetOptions{})
	assert.Assert(t, apierrors.IsNotFound(err))
}

func TestUncontrolledScopeDoesNotWrite(t *testing.T) {
	h := newHarness(t, nil)
	sa := testAccess("local")
	sa.Spec.Issuer, sa.Spec.Certificate = "issuer", "shared"
	h.put(sa)
	h.m.context.(*testContext).excluded = "test"
	h.clearActions()
	assert.NilError(t, h.m.Handle(watchers.ResourceChange{Key: sa.Key()}))
	assert.NilError(t, h.m.certificates.Handle(watchers.ResourceChange{Key: "test/shared"}))
	assert.Equal(t, len(h.mutations()), 0, "unexpected mutations: %v", h.mutations())
	assert.DeepEqual(t, h.status(sa.Name).Status, sa.Status)
}

func TestAccessScopeDoesNotApplyGateway(t *testing.T) {
	h := newHarness(t, nil)
	h.put(testAccess("gateway"))
	h.observe()
	assert.NilError(t, h.run("test/edge"))
	// Access dependencies still reconcile, but do not wake the independent
	// shared-Gateway writer, even for a Gateway-backed Access.
	_, err := h.clients.Kube.CoreV1().Services("test").Get(context.Background(), "edge", metav1.GetOptions{})
	assert.NilError(t, err)
	for _, action := range h.clients.Dynamic.(interface{ Actions() []k8stesting.Action }).Actions() {
		assert.Assert(t, !(action.GetVerb() == "patch" && action.GetResource() == resource.GatewayResource()), "Access wakeup applied the shared Gateway")
	}
}

func TestControlAndPolicyChangesApplyOnNextRun(t *testing.T) {
	h := newHarness(t, nil)
	h.put(testAccess("local"))
	h.m.config.EnabledAccessTypes = []string{"local"}
	c := h.m.context.(*testContext)
	c.excluded = "test"
	assert.NilError(t, h.run("test/edge"))
	_, err := h.clients.Kube.CoreV1().Services("test").Get(context.Background(), "edge", metav1.GetOptions{})
	assert.Assert(t, apierrors.IsNotFound(err))
	c.excluded = ""
	assert.NilError(t, h.run("test/edge"))
	h.observe()
	assert.Assert(t, h.status("edge").IsReady())
	template := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "template", Namespace: "controller",
		Labels: map[string]string{"skupper.io/label-template": "true", "policy": "updated"}}}
	assert.NilError(t, c.policy.Update("controller/template", template))
	assert.NilError(t, h.run("test/edge"))
	svc, err := h.clients.Kube.CoreV1().Services("test").Get(context.Background(), "edge", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, svc.Labels["policy"], "updated")
	h.observe()
	h.noWrites("edge")
}
