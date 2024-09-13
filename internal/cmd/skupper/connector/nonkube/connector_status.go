package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdConnectorStatus struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandConnectorStatusFlags
	Namespace string
	siteName  string
}

func NewCmdConnectorStatus() *CmdConnectorStatus {
	return &CmdConnectorStatus{}
}

func (cmd *CmdConnectorStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdConnectorStatus) ValidateInput(args []string) []error { return nil }
func (cmd *CmdConnectorStatus) InputToOptions()                     {}
func (cmd *CmdConnectorStatus) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdConnectorStatus) WaitUntil() error { return nil }
