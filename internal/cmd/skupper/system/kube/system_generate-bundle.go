package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemGenerateBundle struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Namespace  string
	Flags      *common.CommandSystemGenerateBundleFlags
}

func NewCmdCmdSystemGenerateBundle() *CmdSystemGenerateBundle {

	skupperCmd := CmdSystemGenerateBundle{}

	return &skupperCmd
}

func (cmd *CmdSystemGenerateBundle) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemGenerateBundle) ValidateInput(args []string) error { return nil }

func (cmd *CmdSystemGenerateBundle) InputToOptions() {}

func (cmd *CmdSystemGenerateBundle) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemGenerateBundle) WaitUntil() error { return nil }
