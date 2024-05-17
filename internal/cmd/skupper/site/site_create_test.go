package site

import (
	"testing"
)

func TestCmdSiteCreate_AddFlags(t *testing.T) {
	//TODO

	type test struct {
		name   string
		result bool
	}

	testTable := []test{}
	cmd := setUpCommand()

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd.AddFlags()

		})
	}
}

func TestCmdSiteCreate_ValidateInput(t *testing.T) {
	type test struct {
		name   string
		result bool
	}

	testTable := []test{}
	cmd := setUpCommand()

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd.ValidateInput(nil)

		})
	}
}

func TestCmdSiteCreate_InputToOptions(t *testing.T) {
	type test struct {
		name   string
		result bool
	}

	testTable := []test{}
	cmd := setUpCommand()

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd.InputToOptions(nil)

		})
	}
}

func TestCmdSiteCreate_Run(t *testing.T) {
	type test struct {
		name   string
		result bool
	}

	testTable := []test{}
	cmd := setUpCommand()

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd.Run()

		})
	}
}

func TestCmdSiteCreate_WaitUntilReady(t *testing.T) {
	type test struct {
		name   string
		result bool
	}

	testTable := []test{}
	cmd := setUpCommand()

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd.WaitUntilReady()

		})
	}
}

func setUpCommand() *CmdSiteCreate { return nil }
