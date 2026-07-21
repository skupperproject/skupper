package kube

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/sweeper"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
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
	var validationErrors []error
	numberValidator := validator.NewNumberValidator()
	numberValidator.IncludeZero = false
	if ok, err := numberValidator.Evaluate(cmd.Flags.IdleThreshold); !ok {
		validationErrors = append(validationErrors, fmt.Errorf("idle-threshold is not valid: %s", err))
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdConnSweeper) InputToOptions() {}

func (cmd *CmdConnSweeper) Run() error {
	if cmd.clientErr != nil {
		return cmd.clientErr
	}
	if cmd.KubeClient == nil || cmd.Rest == nil {
		return fmt.Errorf("could not initialize kubernetes client")
	}

	podNames, err := cmd.findRouterPods()
	if err != nil {
		return err
	}

	// Each ready replica has its own connections, so sweep every pod.
	var total sweeper.Result
	var failedPods []string
	for _, podName := range podNames {
		fmt.Printf("=== router pod %s (namespace %s) ===\n", podName, cmd.Namespace)
		res, err := sweeper.Run(sweeper.Config{
			URL:               sweeper.DefaultURL,
			Skmanage:          sweeper.DefaultSkmanage,
			IdleThresholdSecs: cmd.Flags.IdleThreshold,
			Execute:           cmd.Flags.Execute,
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

func (cmd *CmdConnSweeper) findRouterPods() ([]string, error) {
	pods, err := cmd.KubeClient.CoreV1().Pods(cmd.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: routerPodSelector})
	if err != nil {
		return nil, fmt.Errorf("could not list router pods: %w", err)
	}
	var ready []string
	for _, pod := range pods.Items {
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
// container.
func (cmd *CmdConnSweeper) podExecer(podName string) sweeper.Execer {
	return func(argv []string) ([]byte, error) {
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
