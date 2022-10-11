package podman

import (
	"fmt"

	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/system"
)

func (p *PodmanRestClient) Version() (*container.Version, error) {
	systemCli := system.New(p.RestClient, formats)
	info, err := systemCli.SystemInfoLibpod(system.NewSystemInfoLibpodParams())
	if err != nil {
		return nil, fmt.Errorf("error retrieving podman version: %v", err)
	}
	v := &container.Version{}
	if info.Payload.Version != nil {
		v.Server = container.VersionInfo{
			Version:    info.Payload.Version.Version,
			APIVersion: info.Payload.Version.APIVersion,
		}
	}
	return v, nil
}
