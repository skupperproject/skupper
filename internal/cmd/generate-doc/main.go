package main

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/root"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra/doc"
)

func main() {

	utils.HandleError(doc.GenMarkdownTree(root.NewSkupperRootCommand(), "."))

}
