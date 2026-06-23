//go:build integration

// Integration tests for the Skupper kube controller (internal/kube/controller, cmd/controller).
package kubecontrollertest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	kubecontroller "github.com/skupperproject/skupper/internal/kube/controller"
	"gotest.tools/v3/assert"
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
	envTestStopCh  chan struct{}
	envTestStopped chan struct{}
)

func TestMain(m *testing.M) {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:       []string{filepath.Join("..", "..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:   true,
		ControlPlaneStopTimeout: time.Minute,
	}

	var err error
	envTestConfig, err = testEnv.Start()
	if err != nil {
		panic(err)
	}
	defer func() {
		stopSharedController()
		if err := testEnv.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "envtest teardown warning: %v\n", err)
		}
	}()

	if err := startSharedController(); err != nil {
		panic(err)
	}

	m.Run()
}

func startSharedController() error {
	os.Setenv("NAMESPACE", controllerInstallNamespace)
	os.Setenv("CONTROLLER_NAME", "test-controller")
	os.Setenv("SKUPPER_METRICS_DISABLE", "true")

	flags := flag.NewFlagSet("integration-test", flag.ContinueOnError)
	config, err := kubecontroller.BoundConfig(flags)
	if err != nil {
		return err
	}

	clients, err := internalclient.NewClientFromRestConfig(envTestConfig, controllerInstallNamespace)
	if err != nil {
		return err
	}
	envTestClients = clients

	ctx := context.Background()
	_, err = clients.GetKubeClient().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: controllerInstallNamespace},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	ctrl, err := kubecontroller.NewController(clients, config)
	if err != nil {
		return err
	}

	envTestStopCh = make(chan struct{})
	envTestStopped = make(chan struct{})
	go func() {
		_ = ctrl.Run(envTestStopCh)
		close(envTestStopped)
	}()

	time.Sleep(200 * time.Millisecond)
	return nil
}

func stopSharedController() {
	if envTestStopCh == nil {
		return
	}
	close(envTestStopCh)
	select {
	case <-envTestStopped:
	case <-time.After(10 * time.Second):
		fmt.Fprintf(os.Stderr, "controller did not stop within 10s\n")
	}
	time.Sleep(3 * time.Second)
}

type testContext struct {
	t       *testing.T
	clients *internalclient.KubeClient
}

func setup(t *testing.T) *testContext {
	t.Helper()
	return &testContext{t: t, clients: envTestClients}
}

func (tc *testContext) createNamespace(name string) {
	tc.t.Helper()
	ctx := context.Background()
	_, err := tc.clients.GetKubeClient().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		assert.NilError(tc.t, err)
	}
}
