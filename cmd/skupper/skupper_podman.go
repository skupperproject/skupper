package main

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

var notImplementedErr = fmt.Errorf("Not implemented")

var SkupperPodmanCommands = []string{
	"switch", "init", "delete",
}

type SkupperPodman struct {
	site *SkupperPodmanSite
}

func (s *SkupperPodman) Site() SkupperSiteClient {
	return &SkupperPodmanSite{}
}

func (s *SkupperPodman) Service() SkupperServiceClient {
	return &SkupperPodmanService{}
}

func (s *SkupperPodman) Debug() SkupperDebugClient {
	return &SkupperPodmanDebug{}
}

func (s *SkupperPodman) Link() SkupperLinkClient {
	return &SkupperPodmanLink{}
}

func (s *SkupperPodman) Token() SkupperTokenClient {
	return &SkupperPodmanToken{}
}

func (s *SkupperPodman) Network() SkupperNetworkClient {
	return &SkupperPodmanNetwork{}
}

func notImplementedExit() {
	fmt.Println("Not implemented")
	os.Exit(1)
}

func (s *SkupperPodman) NewClient(cmd *cobra.Command, args []string) {
	notImplementedExit()
}

func (s *SkupperPodman) Platform() types.Platform {
	return types.PlatformPodman
}

func (s *SkupperPodman) SupportedCommands() []string {
	return SkupperPodmanCommands
}

func (s *SkupperPodman) Options(cmd *cobra.Command) {
}
