package kube

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/utils/configs"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdVersion struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandVersionFlags
	namespace  string
	output     string
	manifest   configs.ManifestManager
}

func NewCmdVersion() *CmdVersion {

	skupperCmd := CmdVersion{}

	return &skupperCmd
}

func (cmd *CmdVersion) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())

	if err == nil {
		cmd.namespace = cli.Namespace
		cmd.KubeClient = cli.GetKubeClient()
	}
}

func (cmd *CmdVersion) ValidateInput(args []string) []error {
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

	return validationErrors
}

func (cmd *CmdVersion) InputToOptions() {

	mapRunningPods := make(map[string]string)

	if cmd.KubeClient != nil {
		// search for running pods in all namespaces
		runningPodList, err := cmd.KubeClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/part-of=skupper"})
		if err != nil {
			return
		}

		for _, runningPod := range runningPodList.Items {
			for _, container := range runningPod.Status.ContainerStatuses {
				mapRunningPods[container.Name] = container.Image
			}
		}
	}

	if cmd.output != "" {
		cmd.manifest = configs.ManifestManager{Components: images.KubeComponents, EnableSHA: true, RunningPods: mapRunningPods}
	} else {
		cmd.manifest = configs.ManifestManager{Components: images.KubeComponents, EnableSHA: false, RunningPods: mapRunningPods}
	}

}

func (cmd *CmdVersion) Run() error {
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

func (cmd *CmdVersion) WaitUntil() error { return nil }
