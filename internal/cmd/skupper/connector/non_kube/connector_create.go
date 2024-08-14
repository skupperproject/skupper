package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdConnectorCreate struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandConnectorCreateFlags
	Namespace string
	siteName  string
}

func NewCmdConnectorCreate() *CmdConnectorCreate {
	return &CmdConnectorCreate{}
}

func (cmd *CmdConnectorCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdConnectorCreate) AddFlags()                           {}
func (cmd *CmdConnectorCreate) ValidateInput(args []string) []error { return nil }
func (cmd *CmdConnectorCreate) InputToOptions()                     {}
func (cmd *CmdConnectorCreate) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdConnectorCreate) WaitUntil() error { return nil }
