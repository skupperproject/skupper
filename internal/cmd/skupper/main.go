/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package main

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/root"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
)

func main() {

	rootCmd := root.NewSkupperRootCommand()

	err := rootCmd.Execute()
	utils.HandleError(err)
}
