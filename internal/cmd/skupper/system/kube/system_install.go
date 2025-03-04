package kube

import (
	"fmt"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemInstall struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Namespace  string
}

func NewCmdSystemInstall() *CmdSystemInstall {

	skupperCmd := CmdSystemInstall{}

	return &skupperCmd
}

func (cmd *CmdSystemInstall) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemInstall) ValidateInput(args []string) error { return nil }

func (cmd *CmdSystemInstall) InputToOptions() {}

func (cmd *CmdSystemInstall) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemInstall) WaitUntil() error { return nil }
