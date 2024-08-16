package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdSiteUpdate struct {
	CobraCmd           *cobra.Command
	Flags              *common.CommandSiteUpdateFlags
	options            map[string]string
	siteName           string
	serviceAccountName string
	Namespace          string
	linkAccessType     string
	output             string
}

func NewCmdSiteUpdate() *CmdSiteUpdate {

	return &CmdSiteUpdate{}
}

func (cmd *CmdSiteUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdSiteUpdate) ValidateInput(args []string) []error { return nil }
func (cmd *CmdSiteUpdate) InputToOptions()                     {}
func (cmd *CmdSiteUpdate) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdSiteUpdate) WaitUntil() error { return nil }
