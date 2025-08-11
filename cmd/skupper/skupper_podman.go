package main

import (
	"fmt"
	"io"
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
	"service", "expose", "unexpose", "revoke-access", "update", "network",
}

type SkupperPodman struct {
	cliFactory         clientpodman.RestClientFactory
	cli                *clientpodman.PodmanRestClient
	currentSite        *podman.Site
	siteHandlerFactory podman.SiteHandlerFactory
	site               *SkupperPodmanSite
	token              *SkupperPodmanToken
	link               *SkupperPodmanLink
	service            *SkupperPodmanService
	network            *SkupperPodmanNetwork
	exit               exitHandler
	output             io.Writer
}

type exitHandler func(code int)

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
	if s.network != nil {
		return s.network
	}
	s.network = &SkupperPodmanNetwork{
		podman: s,
	}
	return s.network
}

func getCmdEnablePodmanSocket() string {
	if os.Getuid() == 0 {
		return "systemctl enable --now podman.socket"
	}
	return "systemctl --user enable --now podman.socket"
}

func (s *SkupperPodman) NewClient(cmd *cobra.Command, args []string) {
	// endpoint can be provided during init
	var endpoint string
	var isInitCmd bool
	exitOnError := true
	if s.output == nil {
		s.output = os.Stdout
	}
	out := s.output
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
			fmt.Fprintln(out, "error reading current podman endpoint")
			return
		}
		endpoint = podmanCfg.Endpoint
	}
	if s.cliFactory == nil {
		s.cliFactory = clientpodman.NewPodmanClient
	}
	if s.exit == nil {
		s.exit = os.Exit
	}
	c, err := s.cliFactory(endpoint, "")
	if err != nil {
		if exitOnError {
			var recommendation string
			if podmanErr, ok := err.(*clientpodman.Error); ok {
				fmt.Println(podmanErr)
			} else {
				fmt.Fprintf(out, "Podman endpoint is not available: %s",
					utils.DefaultStr(endpoint, clientpodman.GetDefaultPodmanEndpoint()))
				fmt.Fprintln(out)
				recommendation = fmt.Sprintf(`
Recommendation:

	Make sure you have an active podman endpoint available.
	On most systems you can execute:

		%s

	Alternatively you could also create your own service that runs:

		podman system service --time=0 <URI>

	You can get concrete examples through:

		podman help system service`, getCmdEnablePodmanSocket())
				fmt.Fprintln(out, recommendation)
			}
			s.exit(1)
		}
		return
	}
	// only if default endpoint is available or correct endpoint is set
	s.cli = c

	// Ensure that site does not exist on init, but exists for all other commands
	if s.siteHandlerFactory == nil {
		s.siteHandlerFactory = podman.NewSiteHandler
	}
	siteHandler, err := s.siteHandlerFactory(endpoint)
	if err != nil {
		fmt.Fprintf(out, "error verifying existing skupper site - %s", err)
		fmt.Fprintln(out)
		s.exit(1)
		return
	}
	currentSite, err := siteHandler.Get()
	if isInitCmd {
		// Validating if site is already initialized
		if err == nil && currentSite != nil {
			fmt.Fprintf(out, "Skupper has already been initialized for user %s", podman.Username)
			fmt.Fprintln(out)
			s.exit(0)
			return
		}
	} else if !utils.StringSliceContains([]string{"version", "delete"}, cmd.Name()) {
		// All other commands, but version and delete, must stop now
		if err != nil {
			fmt.Fprintf(out, "Skupper is not enabled for user '%s'", podman.Username)
			fmt.Fprintln(out)
			siteHandlerPodman, ok := siteHandler.(*podman.SiteHandler)
			if ok && siteHandlerPodman.AnyResourceLeft() {
				fmt.Fprintln(out, "Reason:", err)
				fmt.Fprintln(out)
				fmt.Fprintln(out, "There are podman resources missing or left from an earlier installation")
				fmt.Fprintln(out, "To clean them up, run: skupper delete")
			}
			s.exit(0)
			return
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
