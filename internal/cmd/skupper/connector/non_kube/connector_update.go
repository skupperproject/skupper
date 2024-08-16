package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdConnectorUpdate struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandConnectorUpdateFlags
	Namespace string
	siteName  string
}

func NewCmdConnectorUpdate() *CmdConnectorUpdate {
	return &CmdConnectorUpdate{}
}

func (cmd *CmdConnectorUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdConnectorUpdate) ValidateInput(args []string) []error { return nil }
func (cmd *CmdConnectorUpdate) InputToOptions()                     {}
func (cmd *CmdConnectorUpdate) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdConnectorUpdate) WaitUntil() error { return nil }
