package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdListenerStatus struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandListenerStatusFlags
	Namespace string
	siteName  string
}

func NewCmdListenerStatus() *CmdListenerStatus {
	return &CmdListenerStatus{}
}

func (cmd *CmdListenerStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdListenerStatus) ValidateInput(args []string) []error { return nil }
func (cmd *CmdListenerStatus) InputToOptions()                     {}
func (cmd *CmdListenerStatus) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdListenerStatus) WaitUntil() error { return nil }
