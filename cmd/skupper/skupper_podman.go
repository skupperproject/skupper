package main

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/spf13/cobra"
)

var notImplementedErr = fmt.Errorf("Not implemented")

var SkupperPodmanCommands = []string{
	"switch", "init", "delete", "status", "version", "token", "link",
	"service", "expose", "unexpose",
}

type SkupperPodman struct {
	cli     *clientpodman.PodmanRestClient
	site    *SkupperPodmanSite
	token   *SkupperPodmanToken
	link    *SkupperPodmanLink
	service *SkupperPodmanService
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
	if s.service != nil {
		return s.service
	}
	s.service = &SkupperPodmanService{
		podman: s,
	}
	return s.service
}

func (s *SkupperPodman) Debug() SkupperDebugClient {
	return &SkupperPodmanDebug{}
}

func (s *SkupperPodman) Link() SkupperLinkClient {
	if s.link != nil {
		return s.link
	}
	s.link = &SkupperPodmanLink{
		podman: s,
	}
	return s.link
}

func (s *SkupperPodman) Token() SkupperTokenClient {
	if s.token != nil {
		return s.token
	}
	s.token = &SkupperPodmanToken{
		podman: s,
	}
	return s.token
}

func (s *SkupperPodman) Network() SkupperNetworkClient {
	return &SkupperPodmanNetwork{}
}

func notImplementedExit() {
	fmt.Println("Not implemented")
	os.Exit(1)
}

func (s *SkupperPodman) NewClient(cmd *cobra.Command, args []string) {
	podmanCfg, err := podman.NewPodmanConfigFileHandler().GetConfig()
	if err != nil {
		return
	}
	c, err := clientpodman.NewPodmanClient(podmanCfg.Endpoint, "")
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
