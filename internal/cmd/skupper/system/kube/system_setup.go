package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemSetup struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandSystemSetupFlags
	Namespace  string
}

func NewCmdSystemSetup() *CmdSystemSetup {

	skupperCmd := CmdSystemSetup{}

	return &skupperCmd
}

func (cmd *CmdSystemSetup) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemSetup) ValidateInput(args []string) []error { return nil }

func (cmd *CmdSystemSetup) InputToOptions() {}

func (cmd *CmdSystemSetup) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemSetup) WaitUntil() error { return nil }
