package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemUnInstall struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Namespace  string
	Flags      *common.CommandSystemUninstallFlags
}

func NewCmdSystemUnInstall() *CmdSystemUnInstall {

	skupperCmd := CmdSystemUnInstall{}

	return &skupperCmd
}

func (cmd *CmdSystemUnInstall) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemUnInstall) ValidateInput(args []string) error { return nil }

func (cmd *CmdSystemUnInstall) InputToOptions() {}

func (cmd *CmdSystemUnInstall) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemUnInstall) WaitUntil() error { return nil }
