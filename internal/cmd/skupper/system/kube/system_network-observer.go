package kube

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdSystemNetworkObserver struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandNetworkObserverFlags
	namespace string
	user      string
	password  string
}

func NewCmdCmdSystemNetworkObserver() *CmdSystemNetworkObserver {
	return &CmdSystemNetworkObserver{}
}

func (cmd *CmdSystemNetworkObserver) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSystemNetworkObserver) ValidateInput(args []string) error {
	return nil
}

func (cmd *CmdSystemNetworkObserver) InputToOptions() {

}

func (cmd *CmdSystemNetworkObserver) Run() error {
	fmt.Println("This command does not support kubernetes platforms.")
	return nil
}

func (cmd *CmdSystemNetworkObserver) WaitUntil() error {
	return nil
}
