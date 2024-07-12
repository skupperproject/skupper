package bundle

import (
	"testing"

	"gotest.tools/assert"
)

func TestEscapeCommand(t *testing.T) {
	var tests = []struct {
		argument string
		expected string
	}{
		{
			argument: "standard-argument",
			expected: "standard-argument",
		},
		{
			argument: "spaces in argument",
			expected: "spaces\\ in\\ argument",
		},
		{
			argument: "special {[ch@ract$rs]}; in \\ argument",
			expected: "special\\ \\{\\[ch@ract\\$rs\\]\\}\\;\\ in\\ \\\\\\ argument",
		},
	}
	for _, test := range tests {
		result := escapeArgument(test.argument)
		assert.Equal(t, test.expected, result)
	}
}
