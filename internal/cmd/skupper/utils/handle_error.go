package utils

import (
	"fmt"
	"strings"
	"syscall"
	"testing"

	"gotest.tools/assert"
)

func HandleError(err error) {
	if err != nil {
		fmt.Println(err)
		syscall.Exit(0)
	}
}

// AssertErrorMessagesMatch asserts that the actual error's message matches the given expected message(s).
// If expected is empty, actual is asserted to be nil.
func AssertErrorMessagesMatch(t *testing.T, expected []string, actual error) {
	if len(expected) == 0 {
		assert.NilError(t, actual)
	} else {
		assert.Error(t, actual, strings.Join(expected, "\n"))
	}
}
