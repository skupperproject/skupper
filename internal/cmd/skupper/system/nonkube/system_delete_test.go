package nonkube

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func TestCmdSystemDelete_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		namespace     string
		args          []string
		flags         *common.CommandSystemDeleteFlags
		expectedError string
	}

	testTable := []test{
		{
			name:          "arguments are not accepted",
			args:          []string{"something"},
			flags:         &common.CommandSystemDeleteFlags{Filename: "-"},
			expectedError: "This command does not accept arguments",
		},
		{
			name:          "flag file is not provided",
			args:          []string{},
			expectedError: "You need to provide a file to delete custom resources or use standard input.\n Example: cat site.yaml | skupper system delete -f -",
		},
		{
			name:          "file does not exist",
			flags:         &common.CommandSystemDeleteFlags{Filename: "file-does-not-exist.json"},
			expectedError: "The file \"file-does-not-exist.json\" does not exist",
		},
		{
			name:          "provided file is not a file but a directory",
			flags:         &common.CommandSystemDeleteFlags{Filename: "."},
			expectedError: "The file has an unsupported extension, it should have one of the following: .yaml, .json\nThe file \".\" is a directory",
		},
		{
			name:          "provided file has an unsupported extension",
			flags:         &common.CommandSystemDeleteFlags{Filename: "file.txt"},
			expectedError: "The file has an unsupported extension, it should have one of the following: .yaml, .json\nThe file \"file.txt\" does not exist",
		},
		{
			name:          "invalid-namespace",
			namespace:     "Invalid",
			flags:         &common.CommandSystemDeleteFlags{Filename: "-"},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdSystemDelete{Flags: test.flags}
			command.Namespace = test.namespace

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdSystemDelete_InputToOptions(t *testing.T) {

	type test struct {
		name              string
		namespace         string
		args              []string
		flags             common.CommandSystemDeleteFlags
		expectedFilename  string
		expectedNamespace string
	}

	testTable := []test{
		{
			name:              "filename is provided",
			namespace:         "",
			flags:             common.CommandSystemDeleteFlags{Filename: "file.yaml"},
			expectedFilename:  "file.yaml",
			expectedNamespace: "default",
		},
		{
			name:              "filename and namespace are provided",
			namespace:         "east",
			flags:             common.CommandSystemDeleteFlags{Filename: "file.yaml"},
			expectedFilename:  "file.yaml",
			expectedNamespace: "east",
		},
		{
			name:              "standard input is provided instead of a file",
			namespace:         "",
			flags:             common.CommandSystemDeleteFlags{Filename: "-"},
			expectedFilename:  "",
			expectedNamespace: "default",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd := newCmdSystemDeleteWithMocks(false)
			cmd.Namespace = test.namespace
			cmd.Flags = &test.flags

			cmd.InputToOptions()

			assert.Check(t, cmd.file == test.expectedFilename)
			assert.Check(t, cmd.Namespace == test.expectedNamespace)
		})
	}
}

func TestCmdSystemDelete_Run(t *testing.T) {
	type test struct {
		name            string
		inputParseFails bool
		errorMessage    string
	}

	testTable := []test{
		{
			name:            "runs without errors",
			inputParseFails: false,
			errorMessage:    "",
		},
		{
			name:            "input parsing fails",
			inputParseFails: true,
			errorMessage:    "Failed parsing the custom resources: fail",
		},
	}

	for _, test := range testTable {
		command := newCmdSystemDeleteWithMocks(test.inputParseFails)

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods

func newCmdSystemDeleteWithMocks(inputParserFails bool) *CmdSystemDelete {

	cmd := &CmdSystemDelete{}
	cmd.CobraCmd = mockCmdSystemDeleteFactory(common.PlatformLinux)

	if inputParserFails {
		cmd.ParseInput = mockInputParserFails
	} else {
		cmd.ParseInput = mockInputParserOK
	}

	return cmd
}

func mockCmdSystemDeleteFactory(configuredPlatform common.Platform) *cobra.Command {

	cmd := common.ConfigureCobraCommand(configuredPlatform, common.SkupperCmdDescription{}, nil, nil)

	testInput := "test input"

	r, w, err := os.Pipe()
	if err != nil {
		slog.Error("failed to create pipe", slog.Any("error", err))
		os.Exit(1)
	}

	// Write to pipe in a goroutine to avoid blocking
	go func() {
		defer w.Close()
		fmt.Fprint(w, testInput)
	}()

	cmd.SetIn(r)

	return cmd
}
