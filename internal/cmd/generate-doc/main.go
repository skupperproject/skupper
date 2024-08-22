package main

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/cmd/skupper/root"
	"github.com/spf13/cobra/doc"
)

func main() {
	path, err := checkArgs(os.Args[1:])
	if err != nil {
		fmt.Printf("%s\n\nUsage: generate-doc ./docsoutput\n", err)
		os.Exit(1)
	}
	utils.HandleError(utils.GenericError, doc.GenMarkdownTree(root.NewSkupperRootCommand(), path))

}

func checkArgs(args []string) (path string, err error) {
	if len(args) != 1 {
		return path, fmt.Errorf("expected single argument")
	}
	path = args[0]
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("output directory %q does not exist", path)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not of type directory", path)
	}
	return path, nil
}
