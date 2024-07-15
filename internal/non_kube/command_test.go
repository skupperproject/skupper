package non_kube

import (
	"testing"

	"gotest.tools/assert"
)

const (
	expectedOutput = `sample_command argument1 argument2 argument3 argument4 argument5 argument6 argument7 \
    argument8 argument9 argument10 argument11 argument12 argument13 argument14 argument15 \
    argument16 argument17 argument18 argument19 argument20 argument21 argument22 argument23 \
    argument24 argument25 argument26 argument27 argument28 argument29
`
)

var (
	commandArgs = []string{
		"sample_command", "argument1", "argument2", "argument3", "argument4", "argument5",
		"argument6", "argument7", "argument8", "argument9", "argument10", "argument11", "argument12",
		"argument13", "argument14", "argument15", "argument16", "argument17", "argument18",
		"argument19", "argument20", "argument21", "argument22", "argument23", "argument24",
		"argument25", "argument26", "argument27", "argument28", "argument29",
	}
)

func TestPrettyPrintCommand(t *testing.T) {
	output := PrettyPrintCommand(commandArgs[0], commandArgs[1:])
	assert.Equal(t, expectedOutput, output)
	shortCommand := PrettyPrintCommand("command", []string{"arg1", "arg2", "arg3"})
	assert.Equal(t, "command arg1 arg2 arg3\n", shortCommand)
}
