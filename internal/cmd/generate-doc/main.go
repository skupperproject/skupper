package main

import (
	"github.com/skupperproject/skupper/internal/cmd/skupperv2/root"
	"github.com/skupperproject/skupper/internal/cmd/skupperv2/utils"
	"github.com/spf13/cobra/doc"
)

func main() {

	utils.HandleError(doc.GenMarkdownTree(root.NewSkupperRootCommand(), "./doc"))

}
