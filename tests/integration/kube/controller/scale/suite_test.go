//go:build integration

// Scale integration tests for the router ConfigMap transport: can the
// controller produce and store the configuration for site shapes at the
// documented safe bounds (thousands of selector-targeted pods and
// Listeners)?
//
// This package deliberately has its own envtest environment, separate from
// tests/integration/kube/controller:
//
//   - The controller is NOT started in TestMain. The tests bulk-load
//     resources first and start the controller afterwards, so that its
//     startup recovery path ingests the entire state and writes each router
//     ConfigMap once. Reconciling tens of thousands of resources one watch
//     event at a time would take forever.
package kubecontrollerscaletest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const controllerInstallNamespace = "skupper-system"

var (
	envTestConfig  *rest.Config
	testEnv        *envtest.Environment
	envTestClients *internalclient.KubeClient
)

func TestMain(m *testing.M) {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:       []string{filepath.Join("..", "..", "..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:   true,
		ControlPlaneStopTimeout: time.Minute,
		ControlPlane: envtest.ControlPlane{
			APIServer: &envtest.APIServer{
				Args: []string{
					"--advertise-address=127.0.0.1",
					// The listener scale test creates thousands of services;
					// the default /24 service CIDR only holds 254.
					"--service-cluster-ip-range=10.96.0.0/16",
				},
			},
		},
	}

	var err error
	envTestConfig, err = testEnv.Start()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := testEnv.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "envtest teardown warning: %v\n", err)
		}
	}()

	// Bulk loading and controller recovery perform tens of thousands of API
	// calls; the default client-side rate limits would dominate the test
	// runtime. This config is shared with the controller started by tests.
	envTestConfig.QPS = 2000
	envTestConfig.Burst = 4000

	envTestClients, err = internalclient.NewClientFromRestConfig(envTestConfig, controllerInstallNamespace)
	if err != nil {
		panic(err)
	}
	_, err = envTestClients.GetKubeClient().CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: controllerInstallNamespace},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		panic(err)
	}

	m.Run()
}
