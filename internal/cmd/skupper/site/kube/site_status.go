package kube

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteStatus struct {
	Client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandSiteStatusFlags
	Namespace string
	output    string
}

func NewCmdSiteStatus() *CmdSiteStatus {

	skupperCmd := CmdSiteStatus{}

	return &skupperCmd
}

func (cmd *CmdSiteStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteStatus) ValidateInput(args []string) error {
	var validationErrors []error
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	if len(args) > 0 {
		return errors.New("this command does not need any arguments")
	}

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

func (cmd *CmdSiteStatus) InputToOptions() {}
func (cmd *CmdSiteStatus) Run() error {

	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		err = utils.HandleMissingCrds(err)
		return err
	}

	if siteList != nil && len(siteList.Items) == 0 {
		fmt.Println("There is no existing Skupper site resource")
		return nil
	}

	if cmd.output != "" {
		for _, site := range siteList.Items {
			encodedOutput, err := utils.Encode(cmd.output, site)
			if err != nil {
				return err
			}
			fmt.Println(encodedOutput)
		}
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
		fmt.Fprintln(writer, "NAME\tSTATUS\tMESSAGE")
		for _, site := range siteList.Items {
			fmt.Fprintf(writer, "%s\t%s\t%s", site.Name, site.Status.StatusType, site.Status.Message)
			fmt.Fprintln(writer)
		}
		writer.Flush()
	}
	return nil
}
func (cmd *CmdSiteStatus) WaitUntil() error { return nil }
