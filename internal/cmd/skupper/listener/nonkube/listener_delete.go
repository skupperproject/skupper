package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdListenerDelete struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandListenerDeleteFlags
	Namespace string
	siteName  string
}

func NewCmdListenerDelete() *CmdListenerDelete {
	return &CmdListenerDelete{}
}

func (cmd *CmdListenerDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdListenerDelete) ValidateInput(args []string) []error { return nil }
func (cmd *CmdListenerDelete) InputToOptions()                     {}
func (cmd *CmdListenerDelete) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdListenerDelete) WaitUntil() error { return nil }
