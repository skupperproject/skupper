package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
)

func TestNewClient(t *testing.T) {

	const didNotExit = -99
	tests := []struct {
		scenario           string
		command            string
		args               []string
		cliFactory         clientpodman.RestClientFactory
		siteHandlerFactory podman.SiteHandlerFactory
		currentSiteLoaded  bool
		exitCode           int
		stdoutContains     string
	}{
		{
			scenario: "init-invalid-endpoint",
			command:  "init",
			args:     []string{"/invalid/podman/endpoint"},
			cliFactory: func(endpoint, basePath string) (*clientpodman.PodmanRestClient, error) {
				return nil, fmt.Errorf("Podman endpoint is not available: %s", endpoint)
			},
			exitCode:       1,
			stdoutContains: "Recommendation:",
		},
		{
			scenario: "init-sitehandler-factory-error",
			command:  "init",
			cliFactory: func(endpoint, basePath string) (*clientpodman.PodmanRestClient, error) {
				return &clientpodman.PodmanRestClient{}, nil
			},
			siteHandlerFactory: func(endpoint string) (domain.SiteHandler, error) {
				return nil, fmt.Errorf("unable to get sitehandler instance [mock]")
			},
			exitCode:       1,
			stdoutContains: "error verifying existing skupper site",
		},
		{
			scenario: "init-already-exists",
			command:  "init",
			cliFactory: func(endpoint, basePath string) (*clientpodman.PodmanRestClient, error) {
				return &clientpodman.PodmanRestClient{}, nil
			},
			siteHandlerFactory: func(endpoint string) (domain.SiteHandler, error) {
				siteHandler := &siteHandlerMock{
					getHook: func() (domain.Site, error) {
						return &podman.Site{}, nil
					},
				}
				return siteHandler, nil
			},
			currentSiteLoaded: false,
			exitCode:          0,
			stdoutContains:    "Skupper has already been initialized for user",
		},
		{
			scenario: "status-ok",
			command:  "status",
			cliFactory: func(endpoint, basePath string) (*clientpodman.PodmanRestClient, error) {
				return &clientpodman.PodmanRestClient{}, nil
			},
			siteHandlerFactory: func(endpoint string) (domain.SiteHandler, error) {
				siteHandler := &siteHandlerMock{
					getHook: func() (domain.Site, error) {
						return &podman.Site{}, nil
					},
				}
				return siteHandler, nil
			},
			currentSiteLoaded: true,
			exitCode:          didNotExit,
		},
		{
			scenario: "status-not-enabled",
			command:  "status",
			cliFactory: func(endpoint, basePath string) (*clientpodman.PodmanRestClient, error) {
				return &clientpodman.PodmanRestClient{}, nil
			},
			siteHandlerFactory: func(endpoint string) (domain.SiteHandler, error) {
				siteHandler := &siteHandlerMock{
					getHook: func() (domain.Site, error) {
						return nil, fmt.Errorf("skupper is not enabled")
					},
				}
				return siteHandler, nil
			},
			currentSiteLoaded: false,
			stdoutContains:    "Skupper is not enabled for user",
			exitCode:          0,
		},
		{
			scenario: "version-not-enabled",
			command:  "version",
			cliFactory: func(endpoint, basePath string) (*clientpodman.PodmanRestClient, error) {
				return &clientpodman.PodmanRestClient{}, nil
			},
			siteHandlerFactory: func(endpoint string) (domain.SiteHandler, error) {
				siteHandler := &siteHandlerMock{
					getHook: func() (domain.Site, error) {
						return nil, fmt.Errorf("skupper is not enabled")
					},
				}
				return siteHandler, nil
			},
			exitCode: didNotExit,
		},
	}

	var s *SkupperPodman
	var lastExitCode int
	stdout := new(bytes.Buffer)
	resetSkupperPodman := func() {
		s = &SkupperPodman{}
		s.exit = func(code int) {
			lastExitCode = code
		}
		s.output = stdout
		lastExitCode = didNotExit
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			// reset test status
			resetSkupperPodman()
			// set factories
			s.cliFactory = test.cliFactory
			s.siteHandlerFactory = test.siteHandlerFactory
			s.NewClient(&cobra.Command{
				Use: test.command,
			}, test.args)
			assert.Equal(t, lastExitCode, test.exitCode)
			assert.Assert(t, strings.Contains(stdout.String(), test.stdoutContains))
			siteLoaded := s.currentSite != nil
			assert.Assert(t, siteLoaded == test.currentSiteLoaded, "expected site to be loaded? %v", test.currentSiteLoaded)
		})
	}
}

type siteHandlerMock struct {
	getHook func() (domain.Site, error)
}

func (sh *siteHandlerMock) Create(s domain.Site) error {
	return fmt.Errorf("not implemented")
}

func (sh *siteHandlerMock) Get() (domain.Site, error) {
	if sh.getHook == nil {
		return nil, fmt.Errorf("not implemented")
	}
	return sh.getHook()
}

func (sh *siteHandlerMock) Delete() error {
	return fmt.Errorf("not implemented")
}

func (sh *siteHandlerMock) Update() error {
	return fmt.Errorf("not implemented")
}

func (sh *siteHandlerMock) RevokeAccess() error {
	return fmt.Errorf("not implemented")
}
