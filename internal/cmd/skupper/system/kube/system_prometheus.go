package kube

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdSystemPrometheus struct {
	CobraCmd *cobra.Command
	Flags    *common.CommandPrometheusFlags
}

func NewCmdSystemPrometheus() *CmdSystemPrometheus {
	return &CmdSystemPrometheus{}
}

func (cmd *CmdSystemPrometheus) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemPrometheus) ValidateInput(args []string) error {
	return nil
}

func (cmd *CmdSystemPrometheus) InputToOptions() {}

func (cmd *CmdSystemPrometheus) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemPrometheus) WaitUntil() error {
	return nil
}
