package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

type SkupperPodmanService struct {
}

func (s *SkupperPodmanService) Create(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) CreateFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) Delete(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) List(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) ListFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) Status(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) NewClient(cmd *cobra.Command, args []string) {}

func (s *SkupperPodmanService) Platform() types.Platform {
	return types.PlatformPodman
}

func (s *SkupperPodmanService) Label(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) Bind(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) BindArgs(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) BindFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) Unbind(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) Expose(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) ExposeArgs(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) ExposeFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) Unexpose(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}
