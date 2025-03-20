package testutils

import (
	"testing"

	"gotest.tools/v3/assert"
)

type inputValidatingCommand interface {
	ValidateInput(args []string) error
}

func CheckValidateInput(t *testing.T, command inputValidatingCommand, expectedError string, args []string) {
	t.Helper()
	actualError := command.ValidateInput(args)

	if expectedError == "" {
		assert.NilError(t, actualError)
	} else {
		assert.Error(t, actualError, expectedError)
	}
}
