package main

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/site_podman"
	"github.com/spf13/cobra"
)

var notImplementedErr = fmt.Errorf("Not implemented")

var SkupperPodmanCommands = []string{
	"switch", "init", "delete", "version",
}

type SkupperPodman struct {
	site *SkupperPodmanSite
	cli  *podman.PodmanRestClient
}

func (s *SkupperPodman) Site() SkupperSiteClient {
	if s.site != nil {
		return s.site
	}
	s.site = &SkupperPodmanSite{
		podman: s,
	}
	return s.site
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
	podmanCfg, err := site_podman.NewPodmanConfigFileHandler().GetConfig()
	if err != nil {
		return
	}
	c, err := podman.NewPodmanClient(podmanCfg.Endpoint, "")
	if err != nil {
		if podmanCfg.Endpoint != "" {
			fmt.Printf("Podman endpoint is not available: %s", podmanCfg.Endpoint)
			os.Exit(1)
		}
		return
	}
	// only if default endpoint is available or correct endpoint is set
	s.cli = c
}

func (s *SkupperPodman) Platform() types.Platform {
	return types.PlatformPodman
}

func (s *SkupperPodman) SupportedCommands() []string {
	return SkupperPodmanCommands
}

func (s *SkupperPodman) Options(cmd *cobra.Command) {
}
