package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

type SkupperPodmanToken struct {
}

func (s *SkupperPodmanToken) Create(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanToken) CreateFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanToken) NewClient(cmd *cobra.Command, args []string) {}

func (s *SkupperPodmanToken) Platform() types.Platform {
	return types.PlatformPodman
}
