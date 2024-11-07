package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdDebug struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandDebugFlags
	Namespace string
}

func NewCmdDebug() *CmdDebug {

	skupperCmd := CmdDebug{}

	return &skupperCmd
}

func (cmd *CmdDebug) NewClient(cobraCommand *cobra.Command, args []string) {

}

func (cmd *CmdDebug) ValidateInput(args []string) []error { return nil }

func (cmd *CmdDebug) InputToOptions() {}

func (cmd *CmdDebug) Run() error { fmt.Println("not yet implemented"); return nil }

func (cmd *CmdDebug) WaitUntil() error { return nil }
