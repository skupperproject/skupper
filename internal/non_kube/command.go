package non_kube

import (
	"bytes"
)

const (
	maxLineLength = 80
	indent        = "   "
	newLinePrefix = " \\\n" + indent
)

func PrettyPrintCommand(command string, args []string) string {
	var lineLength int
	buf := new(bytes.Buffer)
	buf.WriteString(command)
	lineLength = len(command)
	for i, arg := range args {
		buf.WriteString(" ")
		buf.WriteString(arg)
		lineLength += len(arg) + 1
		if lineLength > maxLineLength && i < len(args)-1 {
			buf.WriteString(newLinePrefix)
			lineLength = len(indent)
		}
	}
	buf.WriteString("\n")
	return buf.String()
}
