package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"gotest.tools/v3/assert"
	"testing"
)

func TestCmdSystemStart_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "arg-not-accepted",
			args:           []string{"namespace"},
			expectedErrors: []string{"this command does not accept arguments"},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSystemStart{}
			command.CobraCmd = common.ConfigureCobraCommand(types.PlatformLinux, common.SkupperCmdDescription{}, command, nil)

			actualErrors := command.ValidateInput(test.args)
			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)
			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdSystemStart_InputToOptions(t *testing.T) {

	type test struct {
		name              string
		args              []string
		namespace         string
		expectedNamespace string
	}

	testTable := []test{
		{
			name:              "options-by-default",
			expectedNamespace: "default",
		},
		{
			name:              "namespace-provided",
			args:              []string{"east"},
			namespace:         "east",
			expectedNamespace: "east",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdSystemStopWithMocks(false)
			cmd.Namespace = test.namespace

			cmd.ValidateInput(test.args)
			cmd.InputToOptions()

			assert.Check(t, cmd.Namespace == test.expectedNamespace)

		})
	}
}

func TestCmdSystemStart_Run(t *testing.T) {
	type test struct {
		name           string
		systemCtlFails bool
		errorMessage   string
	}

	testTable := []test{
		{
			name:           "runs ok",
			systemCtlFails: false,
			errorMessage:   "",
		},
		{
			name:           "router start fails",
			systemCtlFails: true,
			errorMessage:   "failed to start router: fail",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemStartWithMocks(test.systemCtlFails)

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error(), err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods

func newCmdSystemStartWithMocks(systemCtlStartFails bool) *CmdSystemStart {

	cmdMock := &CmdSystemStart{
		SystemStart: mockCmdSystemStartSystemCtl,
	}
	if systemCtlStartFails {
		cmdMock.SystemStart = mockCmdSystemStartSystemCtlFails
	}

	return cmdMock
}

func mockCmdSystemStartSystemCtl(namespace string) error { return nil }
func mockCmdSystemStartSystemCtlFails(namespace string) error {
	return fmt.Errorf("fail")
}
