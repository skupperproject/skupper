package kube

import (
	"fmt"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemTeardown struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Namespace  string
}

func NewCmdSystemTeardown() *CmdSystemTeardown {

	skupperCmd := CmdSystemTeardown{}

	return &skupperCmd
}

func (cmd *CmdSystemTeardown) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemTeardown) ValidateInput(args []string) error { return nil }

func (cmd *CmdSystemTeardown) InputToOptions() {}

func (cmd *CmdSystemTeardown) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemTeardown) WaitUntil() error { return nil }
