package main

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

var notImplementedErr = fmt.Errorf("Not implemented")

var SkupperPodmanCommands = []string{
	"switch", "init", "delete", "status", "version", "token", "link",
	"service", "expose", "unexpose", "revoke-access", "update",
}

type SkupperPodman struct {
	cli         *clientpodman.PodmanRestClient
	currentSite *podman.Site
	site        *SkupperPodmanSite
	token       *SkupperPodmanToken
	link        *SkupperPodmanLink
	service     *SkupperPodmanService
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
	// endpoint can be provided during init
	var endpoint string
	var isInitCmd bool
	exitOnError := true
	switch cmd.Name() {
	case "init":
		// require site not present
		if len(args) == 1 {
			endpoint = args[0]
		}
		isInitCmd = true
	case "version":
		exitOnError = false
	default:
		podmanCfg, err := podman.NewPodmanConfigFileHandler().GetConfig()
		if err != nil {
			fmt.Println("error reading current podman endpoint")
			return
		}
		endpoint = podmanCfg.Endpoint
	}

	c, err := clientpodman.NewPodmanClient(endpoint, "")
	if err != nil {
		if exitOnError {
			fmt.Printf("Podman endpoint is not available: %s",
				utils.DefaultStr(endpoint, clientpodman.GetDefaultPodmanEndpoint()))
			fmt.Println()
			os.Exit(1)
		}
		return
	}
	// only if default endpoint is available or correct endpoint is set
	s.cli = c

	// Ensure that site does not exist on init, but exists for all other commands
	siteHandler, err := podman.NewSitePodmanHandler(endpoint)
	if err != nil {
		fmt.Printf("error verifying existing skupper site - %s", err)
		fmt.Println()
		os.Exit(1)
	}
	currentSite, err := siteHandler.Get()
	if isInitCmd {
		// Validating if site is already initialized
		if err == nil && currentSite != nil {
			fmt.Printf("Skupper has already been initialized for user '" + podman.Username + "'.")
			fmt.Println()
			os.Exit(0)
		}
	} else if !utils.StringSliceContains([]string{"version", "delete"}, cmd.Name()) {
		// All other commands, but version and delete, must stop now
		if err != nil {
			fmt.Printf("Skupper is not enabled for user '%s'", podman.Username)
			fmt.Println()
			if siteHandler.AnyResourceLeft() {
				fmt.Println("Reason:", err)
				fmt.Println()
				fmt.Println("There are podman resources missing or left from an earlier installation")
				fmt.Println("To clean them up, run: skupper delete")
			}
			os.Exit(0)
		}
	}
	if currentSite != nil {
		s.currentSite = currentSite.(*podman.Site)
	}
}

func (s *SkupperPodman) Platform() types.Platform {
	return types.PlatformPodman
}

func (s *SkupperPodman) SupportedCommands() []string {
	return SkupperPodmanCommands
}

func (s *SkupperPodman) Options(cmd *cobra.Command) {
}
