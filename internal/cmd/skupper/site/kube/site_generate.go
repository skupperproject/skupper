/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdSiteGenerate struct {
	Client             skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient         kubernetes.Interface
	CobraCmd           *cobra.Command
	Flags              *common.CommandSiteGenerateFlags
	siteName           string
	serviceAccountName string
	Namespace          string
	linkAccessType     string
	output             string
}

func NewCmdSiteGenerate() *CmdSiteGenerate {

	skupperCmd := CmdSiteGenerate{}

	return &skupperCmd
}

func (cmd *CmdSiteGenerate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteGenerate) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(common.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	if cmd.Flags != nil && cmd.Flags.BindHost != "" {
		fmt.Println("Warning: --bind-host flag is not supported on this platform")
	}

	if cmd.Flags != nil && cmd.Flags.SubjectAlternativeNames != nil && len(cmd.Flags.SubjectAlternativeNames) > 0 {
		fmt.Println("Warning: --subject-alternative-names flag is not supported on this platform")
	}

	if len(args) == 0 || args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("site name must not be empty"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command."))
	} else {
		cmd.siteName = args[0]

		ok, err := resourceStringValidator.Evaluate(cmd.siteName)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("site name is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.LinkAccessType != "" {
		ok, err := linkAccessTypeValidator.Evaluate(cmd.Flags.LinkAccessType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link access type is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && !cmd.Flags.EnableLinkAccess && len(cmd.Flags.LinkAccessType) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("for the site to work with this type of linkAccess, the --enable-link-access option must be set to true"))
	}

	if cmd.Flags != nil && cmd.Flags.ServiceAccount != "" {
		svcAccount, err := cmd.KubeClient.CoreV1().ServiceAccounts(cmd.Namespace).Get(context.TODO(), cmd.Flags.ServiceAccount, metav1.GetOptions{})
		if err != nil || svcAccount == nil {
			validationErrors = append(validationErrors, fmt.Errorf("service account name is not valid: %s", err))
		}
	}
	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("format %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSiteGenerate) InputToOptions() {

	cmd.serviceAccountName = cmd.Flags.ServiceAccount

	if cmd.Flags.EnableLinkAccess {
		if cmd.Flags.LinkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.Flags.LinkAccessType
		}
	}

	if cmd.Flags.Output != "" {
		cmd.output = cmd.Flags.Output
	} else {
		cmd.output = "yaml"
	}
}

func (cmd *CmdSiteGenerate) Run() error {

	resource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.siteName,
			Namespace: cmd.Namespace,
		},
		Spec: v2alpha1.SiteSpec{
			ServiceAccount: cmd.serviceAccountName,
			LinkAccess:     cmd.linkAccessType,
		},
	}

	encodedOutput, err := utils.Encode(cmd.output, resource)
	fmt.Println(encodedOutput)
	return err
}

func (cmd *CmdSiteGenerate) WaitUntil() error { return nil }
