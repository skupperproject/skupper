package kube

import (
	"fmt"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemStop struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Namespace  string
}

func NewCmdSystemStop() *CmdSystemStop {

	skupperCmd := CmdSystemStop{}

	return &skupperCmd
}

func (cmd *CmdSystemStop) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemStop) ValidateInput(args []string) []error { return nil }

func (cmd *CmdSystemStop) InputToOptions() {}

func (cmd *CmdSystemStop) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemStop) WaitUntil() error { return nil }
