package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

type SkupperPodmanNetwork struct {
}

func (s *SkupperPodmanNetwork) Status(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanNetwork) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanNetwork) NewClient(cmd *cobra.Command, args []string) {}

func (s *SkupperPodmanNetwork) Platform() types.Platform {
	return types.PlatformPodman
}
