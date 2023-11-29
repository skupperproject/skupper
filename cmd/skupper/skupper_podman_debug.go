package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

type SkupperPodmanDebug struct {
}

func (s *SkupperPodmanDebug) Dump(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanDebug) Events(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanDebug) Service(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanDebug) Policies(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanDebug) NewClient(cmd *cobra.Command, args []string) {}

func (s *SkupperPodmanDebug) Platform() types.Platform {
	return types.PlatformPodman
}
