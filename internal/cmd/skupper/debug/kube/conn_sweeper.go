package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/sweeper"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	routerPodSelector = "app.kubernetes.io/name=skupper-router"
	routerContainer   = "router"
	podExecTimeout    = 30 * time.Second
)

// CmdConnSweeper is the kubernetes entry point for `skupper debug sweep`. It
// finds every ready router pod and runs the sweeper against each one, with
// all commands exec'd inside the router container so they see the pod's own
// network namespace (no port-forward needed).
type CmdConnSweeper struct {
	CobraCmd   *cobra.Command
	Flags      *common.CommandConnSweeperFlags
	KubeClient kubernetes.Interface
	Rest       *restclient.Config
	Namespace  string
	clientErr  error
}

func NewCmdConnSweeper() *CmdConnSweeper {
	return &CmdConnSweeper{}
}

func (cmd *CmdConnSweeper) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.CobraCmd = cobraCommand
	namespaceFlag, _ := cobraCommand.Flags().GetString("namespace")
	contextFlag, _ := cobraCommand.Flags().GetString("context")
	kubeconfigFlag, _ := cobraCommand.Flags().GetString("kubeconfig")

	cli, err := client.NewClient(namespaceFlag, contextFlag, kubeconfigFlag)
	if err != nil {
		cmd.clientErr = fmt.Errorf("failed to initialize kubernetes client: %w", err)
		return
	}
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigFlag != "" {
		loadingRules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigFlag}
	}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: contextFlag},
	)
	restconfig, err := kubeconfig.ClientConfig()
	if err != nil {
		cmd.clientErr = fmt.Errorf("failed to build kubernetes client config: %w", err)
		return
	}

	execConfig := *restconfig
	execConfig.APIPath = "/api"
	execConfig.GroupVersion = &corev1.SchemeGroupVersion
	execConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cmd.Rest = &execConfig
}

func (cmd *CmdConnSweeper) ValidateInput(args []string) error {
	if cmd.Flags.IdleThreshold <= 0 {
		return fmt.Errorf("--idle-threshold must be a positive number of seconds")
	}
	if cmd.clientErr != nil {
		return cmd.clientErr
	}
	if cmd.KubeClient == nil || cmd.Rest == nil {
		return fmt.Errorf("could not initialize kubernetes client")
	}
	return nil
}

func (cmd *CmdConnSweeper) InputToOptions() {}

func (cmd *CmdConnSweeper) Run() error {
	podNames, err := cmd.findRouterPods()
	if err != nil {
		return err
	}

	// Each ready replica has its own connections, so sweep every pod. A
	// failure on one pod (e.g. exec cut off mid-sweep) must not leave the
	// remaining replicas unswept.
	var total sweeper.Result
	var failedPods []string
	for _, podName := range podNames {
		fmt.Printf("=== router pod %s (namespace %s) ===\n", podName, cmd.Namespace)
		res, err := sweeper.Run(sweeper.Config{
			URL:               cmd.Flags.URL,
			Skmanage:          cmd.Flags.Skmanage,
			IdleThresholdSecs: cmd.Flags.IdleThreshold,
			DryRun:            cmd.Flags.DryRun,
			Exec:              cmd.podExecer(podName),
		})
		if err != nil {
			fmt.Printf("sweep of pod %s failed: %v\n", podName, err)
			failedPods = append(failedPods, podName)
			continue
		}
		total.Total += res.Total
		total.Killed += res.Killed
		total.Skipped += res.Skipped
		total.Failed += res.Failed
	}

	if len(podNames) > 1 {
		fmt.Printf("=== all pods: total:%d killed:%d skipped:%d failed:%d ===\n",
			total.Total, total.Killed, total.Skipped, total.Failed)
	}
	if len(failedPods) > 0 {
		return fmt.Errorf("sweep failed on %d of %d router pod(s): %v", len(failedPods), len(podNames), failedPods)
	}
	return nil
}

func (cmd *CmdConnSweeper) WaitUntil() error { return nil }

// findRouterPods returns every router pod whose router container is ready,
// with HA there are multiple replicas and each holds its own connections.
func (cmd *CmdConnSweeper) findRouterPods() ([]string, error) {
	pods, err := cmd.KubeClient.CoreV1().Pods(cmd.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: routerPodSelector})
	if err != nil {
		return nil, fmt.Errorf("could not list router pods: %w", err)
	}
	var ready []string
	for _, pod := range pods.Items {
		// Phase stays "Running" even while a container crash-loops, so
		// require the router container itself to be ready.
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == routerContainer && cs.Ready {
				ready = append(ready, pod.Name)
			}
		}
	}
	if len(ready) == 0 {
		return nil, fmt.Errorf("no ready skupper-router pod found in namespace %q", cmd.Namespace)
	}
	return ready, nil
}

// podExecer returns a sweeper.Execer that runs argv inside the router
// container, where skmanage, python3 and the router's own network namespace
// are all available.
func (cmd *CmdConnSweeper) podExecer(podName string) sweeper.Execer {
	return func(argv []string) ([]byte, error) {
		// ExecCommandInContainer uses the deprecated context-less Stream, so
		// it can't be cancelled directly; run it in a goroutine and give up
		// on the sweep's behalf after podExecTimeout. The buffered channel
		// lets the goroutine finish its send if Stream ever returns.
		type execResult struct {
			out []byte
			err error
		}
		done := make(chan execResult, 1)
		go func() {
			out, err := client.ExecCommandInContainer(argv, podName, routerContainer, cmd.Namespace, cmd.KubeClient, cmd.Rest)
			if err != nil {
				done <- execResult{nil, err}
				return
			}
			done <- execResult{out.Bytes(), nil}
		}()

		select {
		case r := <-done:
			if r.err != nil {
				return nil, fmt.Errorf("exec %q in pod %s failed: %w", argv[0], podName, r.err)
			}
			return r.out, nil
		case <-time.After(podExecTimeout):
			return nil, fmt.Errorf("exec %q in pod %s timed out after %s", argv[0], podName, podExecTimeout)
		}
	}
}
