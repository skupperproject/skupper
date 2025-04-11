package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemApply struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Namespace  string
	Flags      *common.CommandSystemApplyFlags
}

func NewCmdSystemApply() *CmdSystemApply {

	skupperCmd := CmdSystemApply{}

	return &skupperCmd
}

func (cmd *CmdSystemApply) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemApply) ValidateInput(args []string) error { return nil }

func (cmd *CmdSystemApply) InputToOptions() {}

func (cmd *CmdSystemApply) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemApply) WaitUntil() error { return nil }
