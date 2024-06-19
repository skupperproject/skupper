package compat

import (
	"fmt"
	"strings"

	system "github.com/skupperproject/skupper/client/generated/libpod/client/system_compat"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/utils"
)

func (c *CompatClient) Version() (*container.Version, error) {
	systemCli := system.New(c.RestClient, formats)
	params := system.NewSystemVersionParams()
	info, err := systemCli.SystemVersion(params)
	if err != nil {
		return nil, err
	}
	v := &container.Version{}
	if info.Payload != nil {
		v.Server = container.VersionInfo{
			Version:    info.Payload.Version,
			APIVersion: info.Payload.APIVersion,
		}
		v.Arch = info.Payload.Arch
		v.Kernel = info.Payload.KernelVersion
		v.OS = info.Payload.Os
		v.Engine = "docker"
		for _, cmp := range info.Payload.Components {
			if strings.Contains(strings.ToLower(cmp.Name), "podman") {
				v.Engine = "podman"
				if apiVersion, ok := cmp.Details["APIVersion"]; ok {
					v.Server.APIVersion = apiVersion
				}
			}
		}
	}
	return v, nil
}

func (c *CompatClient) Validate() (*container.Version, error) {
	version, err := c.Version()
	if err != nil {
		return nil, fmt.Errorf("container engine is not available on the provided endpoint: %q (unable to verify version) - %w", c.endpoint, err)
	}
	apiVersion := utils.ParseVersion(version.Server.APIVersion)
	if version.Engine == "podman" && apiVersion.Major < 4 {
		return nil, fmt.Errorf("podman version must be 4.0.0 or greater, found: %s", version.Server.APIVersion)
	}
	return version, nil
}
