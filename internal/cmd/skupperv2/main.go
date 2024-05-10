/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package main

import (
	"github.com/skupperproject/skupper/internal/cmd/skupperv2/root"
	"github.com/skupperproject/skupper/internal/cmd/skupperv2/utils"
)

func main() {

	rootCmd := root.NewSkupperRootCommand()

	err := rootCmd.Execute()
	utils.HandleError(err)
}
