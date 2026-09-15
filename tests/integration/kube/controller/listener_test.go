//go:build integration

package kubecontrollertest

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
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

// TestListenerWithoutSite tries to make a Listener when no site has been defined.
// If Skupper does not throw an appropriate error, the test fails.
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

// TestTwoListeners makes two Listeners on a Site. It waits for both to show up,
// then separately waits for the names of both listeners to show up in the Router
// configmap. If either of those don't happen within the timeout, the test fails.
func TestTwoListeners(t *testing.T) {
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

// TestListenerCreateDeleteStorm repeatedly creates and then deletes the same
// Listener, as rapidly as possible, making sure to end up with a creation having
// been the last command. It confirms that we do end up with a Listener,
// its associated Service, and a reference to it in the Router Config.
func TestListenerCreateDeleteStorm(t *testing.T) {
	tc := setup(t)
	namespace := "listener-storm"
	tc.createNamespace(namespace)

	ctx := context.Background()
	listenerName := "storm-listener"
	serviceName := "storm-svc"

	// Make the Site for the test.
	_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Create(ctx, fixtures.Site("mysite", namespace), metav1.CreateOptions{})
	assert.NilError(t, err)

	// This is the Storm!
	// We are deliberately not checking errors here,
	// because we want to see if anything will break.
	const iterations = 100
	for i := 0; i < iterations; i++ {
		listener := listenerWithHostPort(listenerName, namespace, serviceName, 8080)
		_, _ = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Create(ctx, listener, metav1.CreateOptions{})
		// (intentionally ignoring create errors under stress, or treat AlreadyExists as OK)

		if i < iterations-1 {
			_ = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Delete(ctx, listenerName, metav1.DeleteOptions{})
			// (optionally ignore NotFound)
		}
	}

	// After the storm: wait until Listener, Service, and a mention in the Router Config all show up.
	// (They should, because we ended with a create.)
	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {
		// Check for the Listener.
		_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, listenerName, metav1.GetOptions{})
		// If the Get failed only because the object isn’t there yet → keep waiting.
		// If the Get failed for a real reason → fail the test.
		// If the Get succeeded → continue checking other things.
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}

		// Check for the Service.
		_, err = tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}

		// Check for a reference to the Listener in the Router Config.
		routerConfig, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		cfg := routerConfig.Data[types.TransportConfigFile]

		// If everything was found, fall through happy.
		return strings.Contains(cfg, "listener/"+listenerName), nil
	})

	// Final checks :
	// We should still have a Listener
	_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, listenerName, metav1.GetOptions{})
	assert.NilError(t, err)

	// ...and a Service...
	_, err = tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	assert.NilError(t, err)

	// ...and a reference o the Listener in the Router Config.
	routerConfig, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(routerConfig.Data[types.TransportConfigFile], "listener/"+listenerName))
}

// TestIdenticalListenerFailover checks that when two Listeners are identical
// except for their names, only the first one is used, but if the first one
// then goes away, the second one takes over.
// Do this without assuming that the first Listener called for will necessarily
// be the one that gets the binding.
func TestIdenticalListenerFailover(t *testing.T) {
	tc := setup(t)
	namespace := "identical-listener-failover"
	tc.createNamespace(namespace)

	ctx := context.Background()

	_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Sites(namespace).Create(ctx, fixtures.Site("mysite", namespace), metav1.CreateOptions{})
	assert.NilError(t, err)

	// Make Listeners A and B
	a := listenerWithHostPort("listener-a", namespace, "shared-svc", 8080)
	a.Spec.RoutingKey = "shared-key"
	b := listenerWithHostPort("listener-b", namespace, "shared-svc", 8080)
	b.Spec.RoutingKey = "shared-key"

	_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Create(ctx, a, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Create(ctx, b, metav1.CreateOptions{})
	assert.NilError(t, err)

	// Don't assume that it must be Listener A that is the owner of the Service,
	// and Listener B that got the "already exists" error.
	// Figure out explicitly who was the winner and who was the loser.
	var owner, standby string
	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {

		// Get both Listeners so we can look at their status messages.
		la, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, "listener-a", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		lb, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, "listener-b", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}

		// Get the Service and the Router Config so we can
		// see which Listener got the binding.
		svc, err := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "shared-svc", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		routerConfig, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		cfg := routerConfig.Data[types.TransportConfigFile]
		svcOwner := svc.Labels["internal.skupper.io/listener"]

		a_is_the_loser := la.Status.StatusType == skupperv2alpha1.StatusError && strings.Contains(la.Status.Message, "already exists")
		b_is_the_loser := lb.Status.StatusType == skupperv2alpha1.StatusError && strings.Contains(lb.Status.Message, "already exists")

		// The owner is whoever the Service and Router Config both point at.
		// And also the non-owner must be the one with the conflict Error.
		switch {
		case svcOwner == "listener-a" && strings.Contains(cfg, "listener/listener-a") &&
			!strings.Contains(cfg, "listener/listener-b") && b_is_the_loser && !a_is_the_loser:
			owner, standby = "listener-a", "listener-b"
			return true, nil
		case svcOwner == "listener-b" && strings.Contains(cfg, "listener/listener-b") &&
			!strings.Contains(cfg, "listener/listener-a") && a_is_the_loser && !b_is_the_loser:
			owner, standby = "listener-b", "listener-a"
			return true, nil
		default:
			return false, nil
		}
	})

	// Now we know which Listener actually got the binding.
	t.Logf("initial owner=%s standby=%s", owner, standby)

	// Delete the owning Listener, and then
	// wait to see the Not Found error.
	err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Delete(ctx, owner, metav1.DeleteOptions{})
	assert.NilError(t, err)

	waitFor(t, 30*time.Second, 250*time.Millisecond, func() (bool, error) {
		_, err := tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, owner, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
	t.Logf("deleted owner=%s", owner)

	var remaining *skupperv2alpha1.Listener
	waitFor(t, 60*time.Second, 250*time.Millisecond, func() (bool, error) {
		var err error
		remaining, err = tc.clients.GetSkupperClient().SkupperV2alpha1().Listeners(namespace).Get(ctx, standby, metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			// This should never happen. It was already there. But just in case.
			return false, err
		}

		// This is the leftover error from when this Listener was
		// the loser and got the "Already Exists" error.
		// This might take quite a while (like 25 seconds)
		// to clear.
		if remaining.Status.StatusType == skupperv2alpha1.StatusError &&
			strings.Contains(remaining.Status.Message, "already exists") {
			return false, nil
		}

		// Keep waiting until we get the Service,
		// and it shows the standby Listener name as its Listener.
		svc, err := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "shared-svc", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		if svc.Labels["internal.skupper.io/listener"] != standby {
			return false, nil
		}

		// Keep waiting until we get the Router Config, and it
		// contains the name of what was the standby Listener,
		// and no longer contains the name of the original winning Listener.
		routerConfig, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
		if done, err := retryOnNotFound(err); !done {
			return false, err
		}
		cfg := routerConfig.Data[types.TransportConfigFile]
		return strings.Contains(cfg, "listener/"+standby) &&
			!strings.Contains(cfg, "listener/"+owner), nil
	})
	t.Logf("failover complete: %s status=%q message=%q", standby, remaining.Status.StatusType, remaining.Status.Message)

	// Final state: the former standby is now active.
	// Pending/Not Matched is an OK state, because there is no Connector.
	assert.Assert(t, remaining.Status.StatusType != skupperv2alpha1.StatusError ||
		!strings.Contains(remaining.Status.Message, "already exists"))
	configured := meta.FindStatusCondition(remaining.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_CONFIGURED)
	assert.Assert(t, configured != nil)
	assert.Equal(t, configured.Status, metav1.ConditionTrue)

	svc, err := tc.clients.GetKubeClient().CoreV1().Services(namespace).Get(ctx, "shared-svc", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, svc.Labels["internal.skupper.io/listener"], standby)
	assert.Equal(t, len(svc.Spec.Ports), 1)
	assert.Equal(t, svc.Spec.Ports[0].Port, int32(8080))

	routerConfig, err := tc.clients.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, "skupper-router", metav1.GetOptions{})
	assert.NilError(t, err)
	cfg := routerConfig.Data[types.TransportConfigFile]
	assert.Assert(t, strings.Contains(cfg, "listener/"+standby))
	assert.Assert(t, !strings.Contains(cfg, "listener/"+owner))
}
