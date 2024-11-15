package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemStart struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandSystemStartFlags
	Namespace  string
}

func NewCmdSystemStart() *CmdSystemStart {

	skupperCmd := CmdSystemStart{}

	return &skupperCmd
}

func (cmd *CmdSystemStart) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemStart) ValidateInput(args []string) []error { return nil }

func (cmd *CmdSystemStart) InputToOptions() {}

func (cmd *CmdSystemStart) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemStart) WaitUntil() error { return nil }
