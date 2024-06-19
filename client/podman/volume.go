package podman

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
	"github.com/skupperproject/skupper/pkg/container"
)

func (p *PodmanRestClient) VolumeCreate(volume *container.Volume) (*container.Volume, error) {
	if volume.Labels == nil {
		volume.Labels = map[string]string{}
	}
	volume.Labels["application"] = types.AppName
	cli := volumes.New(p.RestClient, formats)
	params := volumes.NewVolumeCreateLibpodParams()
	params.Create = ToVolumeCreateOptions(volume)
	created, err := cli.VolumeCreateLibpod(params)
	if err != nil {
		return nil, err
	}
	return FromCreatedToVolume(created), nil
}

func ToVolumeCreateOptions(v *container.Volume) *models.VolumeCreateOptions {
	nv := &models.VolumeCreateOptions{
		Name:   v.Name,
		Labels: v.Labels,
	}
	return nv
}

func FromCreatedToVolume(created *volumes.VolumeCreateLibpodCreated) *container.Volume {
	v := &container.Volume{
		Name:   created.Payload.Name,
		Source: created.Payload.Mountpoint,
		Labels: created.Payload.Labels,
	}
	return v
}

func (p *PodmanRestClient) VolumeRemove(id string) error {
	v, err := p.VolumeInspect(id)
	if err != nil {
		return err
	}
	if !container.IsOwnedBySkupper(v.GetLabels()) {
		return fmt.Errorf("volume %s is not owned by Skupper", id)
	}
	cli := volumes.New(p.RestClient, formats)
	params := volumes.NewVolumeDeleteLibpodParams()
	params.Name = id
	params.Force = boolTrue()
	_, err = cli.VolumeDeleteLibpod(params)
	if err != nil {
		return err
	}
	return nil
}

func FilesToMounts(c *container.Container) []*models.Mount {
	var mounts []*models.Mount
	for _, fm := range c.FileMounts {
		m := &models.Mount{
			Type:        "bind",
			Source:      fm.Source,
			Destination: fm.Destination,
			Options:     fm.Options,
		}
		mounts = append(mounts, m)
	}
	return mounts
}

func VolumesToNamedVolumes(c *container.Container) []*models.NamedVolume {
	var namedVolumes []*models.NamedVolume
	for _, v := range c.Mounts {
		m := &models.NamedVolume{
			Dest:    v.Destination,
			Name:    v.Name,
			Options: []string{"z", "U"}, // shared between containers
		}
		namedVolumes = append(namedVolumes, m)
	}
	return namedVolumes
}

func (p *PodmanRestClient) VolumeInspect(id string) (*container.Volume, error) {
	cli := volumes.New(p.RestClient, formats)
	params := volumes.NewVolumeInspectLibpodParams()
	params.Name = id
	vi, err := cli.VolumeInspectLibpod(params)
	if err != nil {
		return nil, err
	}
	v := FromInspectToVolume(vi)
	return v, err
}

func FromInspectToVolume(vi *volumes.VolumeInspectLibpodOK) *container.Volume {
	return &container.Volume{
		Name:   vi.Payload.Name,
		Source: vi.Payload.Mountpoint,
		Labels: vi.Payload.Labels,
	}
}

func (p *PodmanRestClient) VolumeList() ([]*container.Volume, error) {
	cli := volumes.New(p.RestClient, formats)
	params := volumes.NewVolumeListLibpodParams()
	pvList, err := cli.VolumeListLibpod(params)
	if err != nil {
		return nil, err
	}
	return FromListToVolume(pvList), nil
}

func FromListToVolume(vi *volumes.VolumeListLibpodOK) []*container.Volume {
	list := []*container.Volume{}
	for _, pv := range vi.Payload {
		list = append(list, &container.Volume{
			Name:   pv.Name,
			Source: pv.Mountpoint,
			Labels: pv.Labels,
		})
	}
	return list
}
