/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/

package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdLinkUpdate struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandLinkUpdateFlags
	Namespace string
	siteName  string
}

func NewCmdLinkUpdate() *CmdLinkUpdate {
	return &CmdLinkUpdate{}
}

func (cmd *CmdLinkUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdLinkUpdate) ValidateInput(args []string) []error { return nil }
func (cmd *CmdLinkUpdate) InputToOptions()                     {}
func (cmd *CmdLinkUpdate) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdLinkUpdate) WaitUntil() error { return nil }
