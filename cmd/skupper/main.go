/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package main

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/root"
)

func main() {

	rootCmd := root.NewSkupperRootCommand()

	err := rootCmd.Execute()
	utils.HandleError(utils.GenericError, err)
}
