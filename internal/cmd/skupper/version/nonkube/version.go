package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdVersion struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandVersionFlags
	Namespace string
}

func NewCmdVersion() *CmdVersion {

	skupperCmd := CmdVersion{}

	return &skupperCmd
}

func (cmd *CmdVersion) NewClient(cobraCommand *cobra.Command, args []string) {

}

func (cmd *CmdVersion) ValidateInput(args []string) []error { return nil }

func (cmd *CmdVersion) InputToOptions() {}

func (cmd *CmdVersion) Run() error { fmt.Println("not yet implemented"); return nil }

func (cmd *CmdVersion) WaitUntil() error { return nil }
