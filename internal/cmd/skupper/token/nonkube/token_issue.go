package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdTokenIssue struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandTokenIssueFlags
	Namespace string
	siteName  string
}

func NewCmdTokenIssue() *CmdTokenIssue {
	return &CmdTokenIssue{}
}

func (cmd *CmdTokenIssue) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdTokenIssue) ValidateInput(args []string) []error { return nil }
func (cmd *CmdTokenIssue) InputToOptions()                     {}
func (cmd *CmdTokenIssue) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdTokenIssue) WaitUntil() error { return nil }
