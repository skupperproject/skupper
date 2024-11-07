package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemReload struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandSystemReloadFlags
	Namespace  string
}

func NewCmdSystemReload() *CmdSystemReload {

	skupperCmd := CmdSystemReload{}

	return &skupperCmd
}

func (cmd *CmdSystemReload) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemReload) ValidateInput(args []string) []error { return nil }

func (cmd *CmdSystemReload) InputToOptions() {}

func (cmd *CmdSystemReload) Run() error {
	fmt.Println("This command is not supported by kubernetes sites.")
	return nil
}

func (cmd *CmdSystemReload) WaitUntil() error { return nil }
