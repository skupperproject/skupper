package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

type SkupperPodmanLink struct {
}

func (s *SkupperPodmanLink) Create(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanLink) CreateFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanLink) Delete(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanLink) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanLink) List(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanLink) ListFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanLink) Status(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanLink) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanLink) NewClient(cmd *cobra.Command, args []string) {}

func (s *SkupperPodmanLink) Platform() types.Platform {
	return types.PlatformPodman
}
