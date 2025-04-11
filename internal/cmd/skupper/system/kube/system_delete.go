package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemDelete struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Namespace  string
	Flags      *common.CommandSystemDeleteFlags
}

func NewCmdSystemDelete() *CmdSystemDelete {

	skupperCmd := CmdSystemDelete{}

	return &skupperCmd
}

func (cmd *CmdSystemDelete) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemDelete) ValidateInput(args []string) error { return nil }

func (cmd *CmdSystemDelete) InputToOptions() {}

func (cmd *CmdSystemDelete) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemDelete) WaitUntil() error { return nil }
