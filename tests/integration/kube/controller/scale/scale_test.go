//go:build integration

package kubecontrollerscaletest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/fixtures"
	kubecontroller "github.com/skupperproject/skupper/internal/kube/controller"
	kubeqdr "github.com/skupperproject/skupper/internal/kube/qdr"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/utils"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// The documented safe bounds for a single site: see the compressed
	// router configuration ADR.
	connectorCount   = 2048
	podsPerConnector = 8
	podCount         = connectorCount * podsPerConnector // 16384 (2^14)

	listenerCount = 10000

	exposingNamespace  = "scale-exposing"
	importingNamespace = "scale-importing"

	configMapSizeLimit = 1024 * 1024
	loadWorkers        = 64
)

// TestRouterConfigAtScaleBounds loads a site with 2,048 Connectors selecting
// 16,384 pods (8 per Connector) and a second site with 10,000 Listeners, then
// starts the controller and verifies that both router ConfigMaps are written
// gzip-compressed, complete, and within the 1MiB ConfigMap limit.
func TestRouterConfigAtScaleBounds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}
	ctx := context.Background()
	kube := envTestClients.GetKubeClient()
	skupper := envTestClients.GetSkupperClient()

	for _, ns := range []string{exposingNamespace, importingNamespace} {
		_, err := kube.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			assert.NilError(t, err)
		}
	}

	_, err := skupper.SkupperV2alpha1().Sites(exposingNamespace).Create(ctx, fixtures.Site("exposing", exposingNamespace), metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = skupper.SkupperV2alpha1().Sites(importingNamespace).Create(ctx, fixtures.Site("importing", importingNamespace), metav1.CreateOptions{})
	assert.NilError(t, err)

	start := time.Now()
	parallel(t, connectorCount, loadWorkers, func(c int) error {
		name := connectorName(c)
		connector := fixtures.Connector(name, exposingNamespace)
		connector.Spec.RoutingKey = name
		connector.Spec.Selector = "app=" + name
		connector.Spec.Port = 8080
		_, err := skupper.SkupperV2alpha1().Connectors(exposingNamespace).Create(ctx, connector, metav1.CreateOptions{})
		return err
	})
	t.Logf("loaded %d connectors in %s", connectorCount, time.Since(start))

	start = time.Now()
	parallel(t, podCount, loadWorkers, func(i int) error {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%d", connectorName(i/podsPerConnector), i%podsPerConnector),
				Labels: map[string]string{"app": connectorName(i / podsPerConnector)},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app", Image: "quay.io/skupper/does-not-run:latest"},
				},
			},
		}
		created, err := kube.CoreV1().Pods(exposingNamespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		// envtest has no kubelet; make the pod eligible for selection
		// (running, ready, with an IP) by writing its status directly.
		ip := podIP(i)
		created.Status = corev1.PodStatus{
			Phase:      corev1.PodRunning,
			PodIP:      ip,
			PodIPs:     []corev1.PodIP{{IP: ip}},
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		}
		_, err = kube.CoreV1().Pods(exposingNamespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
		return err
	})
	t.Logf("loaded %d pods in %s", podCount, time.Since(start))

	start = time.Now()
	parallel(t, listenerCount, loadWorkers, func(i int) error {
		host := fmt.Sprintf("svc-%05d", i)
		listener := fixtures.Listener(host, importingNamespace)
		listener.Spec.Host = host
		listener.Spec.RoutingKey = host
		listener.Spec.Port = 1024 + i
		_, err := skupper.SkupperV2alpha1().Listeners(importingNamespace).Create(ctx, listener, metav1.CreateOptions{})
		return err
	})
	t.Logf("loaded %d listeners in %s", listenerCount, time.Since(start))

	stopController := startController(t)
	defer stopController()

	var exposingConfig, importingConfig *corev1.ConfigMap
	waitFor(t, 5*time.Minute, 2*time.Second, func() (bool, error) {
		cm, err := kube.CoreV1().ConfigMaps(exposingNamespace).Get(ctx, "skupper-router", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if len(cm.BinaryData[types.TransportConfigFileCompressed]) == 0 {
			return false, nil
		}
		config, err := kubeqdr.GetRouterConfigFromConfigMap(cm)
		if err != nil {
			return false, err
		}
		if len(config.Bridges.TcpConnectors) < podCount {
			return false, nil
		}
		exposingConfig = cm
		return true, nil
	})
	t.Logf("exposing site config complete %s after controller start", time.Since(start))

	waitFor(t, 5*time.Minute, 2*time.Second, func() (bool, error) {
		cm, err := kube.CoreV1().ConfigMaps(importingNamespace).Get(ctx, "skupper-router", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if len(cm.BinaryData[types.TransportConfigFileCompressed]) == 0 {
			return false, nil
		}
		config, err := kubeqdr.GetRouterConfigFromConfigMap(cm)
		if err != nil {
			return false, err
		}
		if len(config.Bridges.TcpListeners) < listenerCount {
			return false, nil
		}
		importingConfig = cm
		return true, nil
	})
	t.Logf("importing site config complete %s after controller start", time.Since(start))

	// Exposing site: 16,384 selector-targeted pods -> one tcpConnector per
	// pod, stored compressed, complete, and within the ConfigMap limit.
	assertCompressedWithinLimit(t, exposingConfig)
	config, err := kubeqdr.GetRouterConfigFromConfigMap(exposingConfig)
	assert.NilError(t, err)
	assert.Equal(t, len(config.Bridges.TcpConnectors), podCount)
	for _, i := range []int{0, podCount - 1} {
		name := connectorName(i / podsPerConnector)
		sample := config.Bridges.TcpConnectors[qdr.TcpConnectorNamePrefix+name+"@"+podIP(i)]
		assert.Equal(t, sample.Host, podIP(i))
		assert.Equal(t, sample.Address, name)
		assert.Assert(t, sample.ProcessID != "", "expected pod UID as ProcessID")
	}

	// Importing site: 10,000 Listeners -> one tcpListener each.
	assertCompressedWithinLimit(t, importingConfig)
	config, err = kubeqdr.GetRouterConfigFromConfigMap(importingConfig)
	assert.NilError(t, err)
	assert.Equal(t, len(config.Bridges.TcpListeners), listenerCount)

	// Representative Connectors (first and last) should be marked
	// configured once pod events flow; the exact tcpConnector count above
	// already proves every Connector's pods reached the router config.
	waitFor(t, 2*time.Minute, time.Second, func() (bool, error) {
		for _, c := range []int{0, connectorCount - 1} {
			connector, err := skupper.SkupperV2alpha1().Connectors(exposingNamespace).Get(ctx, connectorName(c), metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if !connector.IsConfigured() {
				return false, nil
			}
		}
		return true, nil
	})
}

func connectorName(c int) string {
	return fmt.Sprintf("workload-%04d", c)
}

func assertCompressedWithinLimit(t *testing.T, cm *corev1.ConfigMap) {
	t.Helper()
	compressed := cm.BinaryData[types.TransportConfigFileCompressed]
	assert.Assert(t, len(compressed) > 0, "expected compressed router config in %s/%s", cm.Namespace, cm.Name)
	_, plain := cm.Data[types.TransportConfigFile]
	assert.Assert(t, !plain, "expected no plain router config alongside compressed in %s/%s", cm.Namespace, cm.Name)
	assert.Assert(t, len(compressed) < configMapSizeLimit,
		"compressed config in %s/%s is %d bytes, over the ConfigMap limit", cm.Namespace, cm.Name, len(compressed))
	raw, err := kubeqdr.GetRouterConfigData(cm)
	assert.NilError(t, err)
	assert.Assert(t, len(raw) > configMapSizeLimit,
		"raw config in %s/%s is only %d bytes; scenario does not exercise the compression bound", cm.Namespace, cm.Name, len(raw))
	t.Logf("%s/%s: raw %d KiB, compressed %d KiB (%.0f%% of limit)",
		cm.Namespace, cm.Name, len(raw)/1024, len(compressed)/1024, 100*float64(len(compressed))/configMapSizeLimit)
}

func podIP(i int) string {
	return fmt.Sprintf("10.64.%d.%d", i/250, 1+i%250)
}

func parallel(t *testing.T, n, workers int, fn func(i int) error) {
	t.Helper()
	indices := make(chan int)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range indices {
				if err := fn(i); err != nil {
					select {
					case errs <- fmt.Errorf("item %d: %w", i, err):
					default:
					}
				}
			}
		})
	}
	for i := range n {
		indices <- i
	}
	close(indices)
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, timeout, interval time.Duration, fn func() (bool, error)) {
	t.Helper()
	assert.NilError(t, utils.Retry(interval, int(timeout/interval), fn))
}

func startController(t *testing.T) func() {
	t.Helper()
	os.Setenv("NAMESPACE", controllerInstallNamespace)
	os.Setenv("CONTROLLER_NAME", "test-controller")
	os.Setenv("SKUPPER_METRICS_DISABLE", "true")

	flags := flag.NewFlagSet("scale-integration-test", flag.ContinueOnError)
	config, err := kubecontroller.BoundConfig(flags)
	assert.NilError(t, err)

	ctrl, err := kubecontroller.NewController(envTestClients, config)
	assert.NilError(t, err)

	stopCh := make(chan struct{})
	stopped := make(chan error, 1)
	go func() {
		stopped <- ctrl.Run(stopCh)
	}()
	return func() {
		close(stopCh)
		select {
		case err := <-stopped:
			if err != nil {
				t.Logf("controller stopped with error: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Log("controller did not stop within 30s")
		}
	}
}
