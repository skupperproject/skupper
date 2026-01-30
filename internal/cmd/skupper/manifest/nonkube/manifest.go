package nonkube

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/pkg/nonkube/api"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/images"
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
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}
}

func (cmd *CmdManifest) ValidateInput(args []string) error {
	var validationErrors []error
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	namespaceStringValidator := validator.NamespaceStringValidator()

	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.output = cmd.Flags.Output
		}
	}

	if cmd.namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	_, err := os.Stat(api.GetHostNamespaceHome(cmd.namespace))
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("there is no definition for namespace %q", cmd.namespace))
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdManifest) InputToOptions() {

	mapRunningPods := make(map[string]string)

	if cmd.namespace == "" {
		cmd.namespace = "default"
	}

	cli, err := internalclient.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")

	if err == nil {
		containerName := cmd.namespace + "-skupper-router"
		if container, err := cli.ContainerInspect(containerName); err == nil {
			mapRunningPods[container.Name] = container.Image
		}
	}

	if cmd.output != "" {
		cmd.manifest = configs.ManifestManager{Components: images.NonKubeComponents, EnableSHA: true, RunningPods: mapRunningPods}
	} else {
		cmd.manifest = configs.ManifestManager{Components: images.NonKubeComponents, EnableSHA: false, RunningPods: mapRunningPods}
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
