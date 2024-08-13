/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdLinkGenerate struct {
	CobraCmd  *cobra.Command
	Namespace string
	siteName  string
	Flags     *common.CommandLinkGenerateFlags
}

func NewCmdLinkGenerate() *CmdLinkGenerate {
	return &CmdLinkGenerate{}
}

func (cmd *CmdLinkGenerate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdLinkGenerate) AddFlags()                           {}
func (cmd *CmdLinkGenerate) ValidateInput(args []string) []error { return nil }
func (cmd *CmdLinkGenerate) InputToOptions()                     {}
func (cmd *CmdLinkGenerate) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdLinkGenerate) WaitUntil() error { return nil }
