package kube

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/images"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/configs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdManifest struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandVersionFlags
	namespace  string
	output     string
	manifest   configs.ManifestManager
}

func NewCmdManifest() *CmdManifest {

	skupperCmd := CmdManifest{}

	return &skupperCmd
}

func (cmd *CmdManifest) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())

	if err == nil {
		cmd.namespace = cli.Namespace
		cmd.KubeClient = cli.GetKubeClient()
	}
}

func (cmd *CmdManifest) ValidateInput(args []string) error {
	var validationErrors []error
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.output = cmd.Flags.Output
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdManifest) InputToOptions() {

	mapRunningPods := make(map[string]string)

	if cmd.KubeClient != nil {
		// search for running pods in the current namespace
		runningPodList, err := cmd.KubeClient.CoreV1().Pods(cmd.namespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/part-of in (skupper, skupper-network-observer)"})
		if err == nil {
			for _, runningPod := range runningPodList.Items {
				for _, container := range runningPod.Status.ContainerStatuses {
					mapRunningPods[container.Name] = container.Image
				}
			}
		}
	}

	if cmd.output != "" {
		cmd.manifest = configs.ManifestManager{Components: images.KubeComponents, EnableSHA: true, RunningPods: mapRunningPods}
	} else {
		cmd.manifest = configs.ManifestManager{Components: images.KubeComponents, EnableSHA: false, RunningPods: mapRunningPods}
	}
}

func (cmd *CmdManifest) Run() error {
	files := cmd.manifest.GetConfiguredManifest()

	if cmd.output != "" {
		encodedOutput, err := utils.Encode(cmd.output, files)
		if err != nil {
			return err
		}
		fmt.Println(encodedOutput)
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
		_, _ = fmt.Fprintln(tw, fmt.Sprintf("%s\t%s", "COMPONENT", "VERSION"))

		for _, file := range files.Components {
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%s", file.Component, file.Version))
		}
		_ = tw.Flush()
	}
	return nil
}

func (cmd *CmdManifest) WaitUntil() error { return nil }
