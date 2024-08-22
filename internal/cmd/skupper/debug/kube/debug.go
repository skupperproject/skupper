package kube

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdDebug struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandDebugFlags
	Namespace  string
}

func NewCmdDebug() *CmdDebug {

	skupperCmd := CmdDebug{}

	return &skupperCmd
}

func (cmd *CmdDebug) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	if err == nil {
		cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
		cmd.KubeClient = cli.GetKubeClient()
		cmd.Namespace = cli.Namespace
	}
}

func (cmd *CmdDebug) ValidateInput(args []string) error { return nil }

func (cmd *CmdDebug) InputToOptions() {}

func (cmd *CmdDebug) Run() error { fmt.Println("not yet implemented"); return nil }

func (cmd *CmdDebug) WaitUntil() error { return nil }
