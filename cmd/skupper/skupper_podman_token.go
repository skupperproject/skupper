package main

import (
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

type SkupperPodmanToken struct {
	podman      *SkupperPodman
	ingressHost string
}

func (s *SkupperPodmanToken) Create(cmd *cobra.Command, args []string) error {
	subject := clientIdentity
	secretFile := args[0]

	// Determining ingress host
	siteHandler, err := podman.NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("error retrieving site information - %w", err)
	}
	site, err := siteHandler.Get()
	if err != nil {
		return fmt.Errorf("error inspecting site - %w", err)
	}
	sitePodman := site.(*podman.SitePodman)
	if sitePodman.IsEdge() {
		return fmt.Errorf("Edge configuration cannot accept connections")
	}
	ingressHost := utils.DefaultStr(s.ingressHost, sitePodman.IngressBindHost)
	if ingressHost == "" {
		return fmt.Errorf("Unable to determine ingress host (use --ingress-host)")
	}
	info := &domain.TokenCertInfo{
		InterRouterHost: ingressHost,
		InterRouterPort: strconv.Itoa(sitePodman.IngressBindInterRouterPort),
		EdgeHost:        ingressHost,
		EdgePort:        strconv.Itoa(sitePodman.IngressBindEdgePort),
	}

	// Retrieving CA
	credHandler := podman.NewPodmanCredentialHandler(s.podman.cli)

	// Creating secret
	tokenHandler := &podman.TokenCertHandlerPodman{}
	return tokenHandler.Create(secretFile, subject, info, sitePodman, credHandler)
}

func (s *SkupperPodmanToken) CreateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&s.ingressHost, "ingress-host", "", "", "Hostname or alias by which the ingress route or proxy can be reached")
}

func (s *SkupperPodmanToken) NewClient(cmd *cobra.Command, args []string) {
	s.podman.NewClient(cmd, args)
}

func (s *SkupperPodmanToken) Platform() types.Platform {
	return s.podman.Platform()
}
