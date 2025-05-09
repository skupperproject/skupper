package command

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/cli"
	"github.com/spf13/cobra"
)

type Check interface {
	Name() string
	CheckDescription() string
	Example() string
	Run(cli.Reporter, *cobra.Command) error
	Dependencies() []*Check
}

type BaseCheck struct {
	name             string
	checkDescription string
	example          string
	dependencies     []*Check
}

func NewBaseCheckCommand(name, checkDescription string, dependencies ...*Check) BaseCheck {
	return BaseCheck{name: name, checkDescription: checkDescription, dependencies: dependencies}
}

func (bd *BaseCheck) Name() string {
	return bd.name
}

func (bd *BaseCheck) CheckDescription() string {
	return bd.checkDescription
}

func (bd *BaseCheck) Example() string {
	return bd.example
}

func (bd *BaseCheck) Dependencies() []*Check {
	return bd.dependencies
}
