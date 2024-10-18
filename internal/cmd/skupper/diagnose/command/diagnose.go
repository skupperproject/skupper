package command

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/cli"
	"github.com/spf13/cobra"
)

type Diagnose interface {
	Name() string
	CheckDescription() string
	Example() string
	Run(cli.Reporter, *cobra.Command) error
	Dependencies() []*Diagnose
}

type BaseDiagnose struct {
	name             string
	checkDescription string
	example          string
	dependencies     []*Diagnose
}

func NewBaseDiagnoseCommand(name, checkDescription string, dependencies ...*Diagnose) BaseDiagnose {
	return BaseDiagnose{name: name, checkDescription: checkDescription, dependencies: dependencies}
}

func (bd *BaseDiagnose) Name() string {
	return bd.name
}

func (bd *BaseDiagnose) CheckDescription() string {
	return bd.checkDescription
}

func (bd *BaseDiagnose) Example() string {
	return bd.example
}

func (bd *BaseDiagnose) Dependencies() []*Diagnose {
	return bd.dependencies
}
