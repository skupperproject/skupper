package kube

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// CmdConnSweeper is the kubernetes entry point for `skupper debug conn`. It
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
	if err := sweeper.ValidatePorts(cmd.Flags.Ports); err != nil {
		validationErrors = append(validationErrors, err)
	}
	if _, err := sweeper.NormalizeStates(cmd.Flags.States); err != nil {
		validationErrors = append(validationErrors, err)
	}
	if err := sweeper.ValidateOutput(cmd.Flags.Output); err != nil {
		validationErrors = append(validationErrors, err)
	}
	if cmd.Flags.ListPorts {
		if cmd.Flags.Execute {
			validationErrors = append(validationErrors, fmt.Errorf("--execute cannot be used with --list-ports: listing ports never closes connections"))
		}
		if len(cmd.Flags.States) > 0 {
			validationErrors = append(validationErrors, fmt.Errorf("--state cannot be used with --list-ports: port listing does not query kernel sockets"))
		}
		return errors.Join(validationErrors...)
	}
	numberValidator := validator.NewNumberValidator()
	numberValidator.IncludeZero = false
	if ok, err := numberValidator.Evaluate(cmd.Flags.IdleThreshold); !ok {
		validationErrors = append(validationErrors, fmt.Errorf("idle-threshold is not valid: %s", err))
	}
	if int64(cmd.Flags.IdleThreshold) > sweeper.MaxIdleThreshold {
		validationErrors = append(validationErrors, fmt.Errorf("idle-threshold is too large (max %d seconds)", sweeper.MaxIdleThreshold))
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

	if cmd.Flags.ListPorts {
		return cmd.listPorts(podNames)
	}

	jsonMode := sweeper.NormalizeOutput(cmd.Flags.Output) == sweeper.OutputJSON
	var total sweeper.Result
	var allReports []sweeper.ConnReport
	var failedPods []string
	for _, podName := range podNames {
		cmd.diagPodHeader(podName, jsonMode)
		res, err := sweeper.Run(sweeper.Config{
			URL:               sweeper.DefaultURL,
			Skmanage:          sweeper.DefaultSkmanage,
			IdleThresholdSecs: cmd.Flags.IdleThreshold,
			Execute:           cmd.Flags.Execute,
			Ports:             cmd.Flags.Ports,
			States:            cmd.Flags.States,
			RoutingKeys:       cmd.Flags.RoutingKeys,
			Output:            cmd.Flags.Output,
			Exec:              cmd.podExecer(podName),
		})
		if err != nil {
			cmd.diagf(jsonMode, "conn inspection of pod %s failed: %v\n", podName, err)
			failedPods = append(failedPods, podName)
			continue
		}
		total.Total += res.Total
		total.Killed += res.Killed
		total.Skipped += res.Skipped
		total.Failed += res.Failed
		allReports = append(allReports, res.Reports...)
	}

	if jsonMode {
		if allReports == nil {
			allReports = []sweeper.ConnReport{}
		}
		if err := sweeper.WriteJSON(os.Stdout, allReports); err != nil {
			return err
		}
	} else if len(podNames) > 1 {
		fmt.Printf("=== all pods: total:%d killed:%d skipped:%d failed:%d ===\n",
			total.Total, total.Killed, total.Skipped, total.Failed)
	}
	if len(failedPods) > 0 {
		return fmt.Errorf("conn inspection failed on %d of %d router pod(s): %v", len(failedPods), len(podNames), failedPods)
	}
	if total.Failed > 0 {
		return fmt.Errorf("%d idle connection(s) failed to close (%d closed)", total.Failed, total.Killed)
	}
	return nil
}

func (cmd *CmdConnSweeper) WaitUntil() error { return nil }

// listPorts prints a port table per router pod (text), or one merged JSON
// document for all pods.
func (cmd *CmdConnSweeper) listPorts(podNames []string) error {
	jsonMode := sweeper.NormalizeOutput(cmd.Flags.Output) == sweeper.OutputJSON
	var perPod [][]sweeper.PortStat
	var failedPods []string
	for _, podName := range podNames {
		cmd.diagPodHeader(podName, jsonMode)
		stats, err := sweeper.ListPorts(sweeper.Config{
			URL:         sweeper.DefaultURL,
			Skmanage:    sweeper.DefaultSkmanage,
			Ports:       cmd.Flags.Ports,
			RoutingKeys: cmd.Flags.RoutingKeys,
			Output:      cmd.Flags.Output,
			Exec:        cmd.podExecer(podName),
		})
		if err != nil {
			cmd.diagf(jsonMode, "could not list ports for pod %s: %v\n", podName, err)
			if !jsonMode {
				fmt.Println()
			}
			failedPods = append(failedPods, podName)
			continue
		}
		perPod = append(perPod, stats)
		if !jsonMode {
			if err := sweeper.PrintPortStatsToStdout(stats, cmd.Flags.Ports, cmd.Flags.Output); err != nil {
				return err
			}
			fmt.Println()
		}
	}

	if jsonMode {
		merged := sweeper.MergePortStats(perPod...)
		if err := sweeper.PrintPortStatsToStdout(merged, cmd.Flags.Ports, cmd.Flags.Output); err != nil {
			return err
		}
	} else if len(perPod) > 1 {
		fmt.Println("=== all pods ===")
		if err := sweeper.PrintPortStatsToStdout(sweeper.MergePortStats(perPod...), cmd.Flags.Ports, cmd.Flags.Output); err != nil {
			return err
		}
	}
	if len(failedPods) > 0 {
		return fmt.Errorf("could not list ports on %d of %d router pod(s): %v", len(failedPods), len(podNames), failedPods)
	}
	return nil
}

func (cmd *CmdConnSweeper) diagPodHeader(podName string, jsonMode bool) {
	cmd.diagf(jsonMode, "=== router pod %s (namespace %s) ===\n", podName, cmd.Namespace)
}

func (cmd *CmdConnSweeper) diagf(jsonMode bool, format string, args ...any) {
	if jsonMode {
		fmt.Fprintf(os.Stderr, format, args...)
		return
	}
	fmt.Printf(format, args...)
}

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
