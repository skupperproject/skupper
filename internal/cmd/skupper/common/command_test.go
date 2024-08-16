package common

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
	"testing"
)

type MockSkupperCommand struct {
	CalledNewClient      bool
	CalledValidateInput  bool
	CalledInputToOptions bool
	CalledRun            bool
	CalledWaitUntil      bool
}

func (m *MockSkupperCommand) NewClient(cmd *cobra.Command, args []string) {
	m.CalledNewClient = true
}

func (m *MockSkupperCommand) ValidateInput(args []string) []error {
	m.CalledValidateInput = true
	return nil
}

func (m *MockSkupperCommand) InputToOptions() {
	m.CalledInputToOptions = true
}

func (m *MockSkupperCommand) Run() error {
	m.CalledRun = true
	return nil
}

func (m *MockSkupperCommand) WaitUntil() error {
	m.CalledWaitUntil = true
	return nil
}

func TestConfigureCobraCommand(t *testing.T) {
	t.Run("Test with kubernetes platform", func(t *testing.T) {
		kubeCmd := &MockSkupperCommand{}
		nonKubeCmd := &MockSkupperCommand{}
		desc := SkupperCmdDescription{
			Use:     "testuse",
			Short:   "testshort",
			Long:    "testlong",
			Example: "testexample",
		}

		result := ConfigureCobraCommand(types.PlatformKubernetes, desc, kubeCmd, nonKubeCmd)

		// After executing the returned cobra.Command,
		// the corresponding methods on the correct SkupperCommand should have been called
		err := result.Execute()
		assert.Check(t, err)

		assert.Check(t, kubeCmd.CalledNewClient)
		assert.Check(t, kubeCmd.CalledValidateInput)
		assert.Check(t, kubeCmd.CalledInputToOptions)
		assert.Check(t, kubeCmd.CalledRun)
		assert.Check(t, kubeCmd.CalledWaitUntil)

		// Ensure nonKubeCmd wasn't called
		assert.Check(t, !nonKubeCmd.CalledNewClient)
		assert.Check(t, !nonKubeCmd.CalledValidateInput)
		assert.Check(t, !nonKubeCmd.CalledInputToOptions)
		assert.Check(t, !nonKubeCmd.CalledRun)
		assert.Check(t, !nonKubeCmd.CalledWaitUntil)
	})

	t.Run("Test with non kubernetes platform", func(t *testing.T) {
		kubeCmd := &MockSkupperCommand{}
		nonKubeCmd := &MockSkupperCommand{}
		desc := SkupperCmdDescription{
			Use:     "testuse",
			Short:   "testshort",
			Long:    "testlong",
			Example: "testexample",
		}

		var selectedPlatform string
		result := ConfigureCobraCommand(types.PlatformKubernetes, desc, kubeCmd, nonKubeCmd)
		result.Flags().StringVarP(&selectedPlatform, FlagNamePlatform, "p", "docker", FlagDescPlatform)

		// After executing the returned cobra.Command,
		// the corresponding methods on the correct SkupperCommand should have been called
		err := result.Execute()
		assert.Check(t, err)

		// Ensure KubeCmd wasn't called
		assert.Check(t, !kubeCmd.CalledNewClient)
		assert.Check(t, !kubeCmd.CalledValidateInput)
		assert.Check(t, !kubeCmd.CalledInputToOptions)
		assert.Check(t, !kubeCmd.CalledRun)
		assert.Check(t, !kubeCmd.CalledWaitUntil)

		assert.Check(t, nonKubeCmd.CalledNewClient)
		assert.Check(t, nonKubeCmd.CalledValidateInput)
		assert.Check(t, nonKubeCmd.CalledInputToOptions)
		assert.Check(t, nonKubeCmd.CalledRun)
		assert.Check(t, nonKubeCmd.CalledWaitUntil)
	})

	t.Run("Test with unsupported platform", func(t *testing.T) {
		kubeCmd := &MockSkupperCommand{}
		nonKubeCmd := &MockSkupperCommand{}
		desc := SkupperCmdDescription{
			Use:     "testuse",
			Short:   "testshort",
			Long:    "testlong",
			Example: "testexample",
		}

		var selectedPlatform string
		result := ConfigureCobraCommand(types.PlatformKubernetes, desc, kubeCmd, nonKubeCmd)
		result.Flags().StringVarP(&selectedPlatform, FlagNamePlatform, "p", "unsupported", FlagDescPlatform)

		// After executing the returned cobra.Command,
		// the corresponding methods on the correct SkupperCommand should have been called
		err := result.Execute()
		assert.Check(t, err.Error() == "platform \"unsupported\" not supported")

		assert.Check(t, !kubeCmd.CalledNewClient)
		assert.Check(t, !kubeCmd.CalledValidateInput)
		assert.Check(t, !kubeCmd.CalledInputToOptions)
		assert.Check(t, !kubeCmd.CalledRun)
		assert.Check(t, !kubeCmd.CalledWaitUntil)

		assert.Check(t, !nonKubeCmd.CalledNewClient)
		assert.Check(t, !nonKubeCmd.CalledValidateInput)
		assert.Check(t, !nonKubeCmd.CalledInputToOptions)
		assert.Check(t, !nonKubeCmd.CalledRun)
		assert.Check(t, !nonKubeCmd.CalledWaitUntil)
	})
}
