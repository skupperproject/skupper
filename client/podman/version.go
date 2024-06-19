package podman

import (
	"github.com/skupperproject/skupper/client/generated/libpod/client/system"
	"github.com/skupperproject/skupper/pkg/container"
)

func (p *PodmanRestClient) Version() (*container.Version, error) {
	systemCli := system.New(p.RestClient, formats)
	info, err := systemCli.SystemInfoLibpod(system.NewSystemInfoLibpodParams())
	if err != nil {
		return nil, err
	}
	v := &container.Version{}
	if info.Payload.Version != nil {
		v.Server = container.VersionInfo{
			Version:    info.Payload.Version.Version,
			APIVersion: info.Payload.Version.APIVersion,
		}
		v.Hostname = info.Payload.Host.Hostname
		v.Arch = info.Payload.Host.Arch
		v.Kernel = info.Payload.Host.Kernel
		v.OS = info.Payload.Host.OS
	}

	return v, nil
}
