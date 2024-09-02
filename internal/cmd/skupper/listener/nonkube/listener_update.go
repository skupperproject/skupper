package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdListenerUpdate struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandListenerUpdateFlags
	Namespace string
	siteName  string
}

func NewCmdListenerUpdate() *CmdListenerUpdate {
	return &CmdListenerUpdate{}
}

func (cmd *CmdListenerUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdListenerUpdate) ValidateInput(args []string) []error { return nil }
func (cmd *CmdListenerUpdate) InputToOptions()                     {}
func (cmd *CmdListenerUpdate) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdListenerUpdate) WaitUntil() error { return nil }
