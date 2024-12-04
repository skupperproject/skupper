package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"gotest.tools/v3/assert"

	"testing"
)

func TestCmdSystemStop_ValidateInput(t *testing.T) {
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

			command := &CmdSystemStop{}
			command.CobraCmd = common.ConfigureCobraCommand(types.PlatformSystemd, common.SkupperCmdDescription{}, command, nil)

			actualErrors := command.ValidateInput(test.args)
			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)
			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdSystemStop_InputToOptions(t *testing.T) {

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

func TestCmdSystemStop_Run(t *testing.T) {
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
			name:           "stop router fails",
			systemCtlFails: true,
			errorMessage:   "failed to stop router: fail",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemStopWithMocks(test.systemCtlFails)

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

func newCmdSystemStopWithMocks(systemCtlStopFails bool) *CmdSystemStop {

	cmdMock := &CmdSystemStop{
		SystemStop: mockCmdSystemStopSystemCtl,
	}
	if systemCtlStopFails {
		cmdMock.SystemStop = mockCmdSystemStopSystemCtlFails
	}

	return cmdMock
}

func mockCmdSystemStopSystemCtl(namespace string) error { return nil }
func mockCmdSystemStopSystemCtlFails(namespace string) error {
	return fmt.Errorf("fail")
}
