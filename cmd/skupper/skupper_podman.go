package main

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

type SkupperPodman struct {
}

func (s *SkupperPodman) NewClient(cmd *cobra.Command, args []string) {
	fmt.Println("Not implemented")
	os.Exit(1)
}

func (s *SkupperPodman) Platform() types.Platform {
	// TODO implement me
	panic("implement me")
}

func (s *SkupperPodman) SupportedCommands() []string {
	// TODO implement me
	panic("implement me")
}

func (s *SkupperPodman) Options(cmd *cobra.Command) {
	// TODO implement me
	panic("implement me")
}

func (s *SkupperPodman) Init(cmd *cobra.Command, args []string) error {
	// TODO implement me
	panic("implement me")
}

func (s *SkupperPodman) InitFlags(cmd *cobra.Command) {
	// TODO implement me
	panic("implement me")
}

func (s *SkupperPodman) DebugDump(cmd *cobra.Command, args []string) error {
	// TODO implement me
	panic("implement me")
}
