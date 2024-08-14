package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdConnectorDelete struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandConnectorDeleteFlags
	Namespace string
	siteName  string
}

func NewCmdConnectorDelete() *CmdConnectorDelete {
	return &CmdConnectorDelete{}
}

func (cmd *CmdConnectorDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdConnectorDelete) AddFlags()                           {}
func (cmd *CmdConnectorDelete) ValidateInput(args []string) []error { return nil }
func (cmd *CmdConnectorDelete) InputToOptions()                     {}
func (cmd *CmdConnectorDelete) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdConnectorDelete) WaitUntil() error { return nil }
