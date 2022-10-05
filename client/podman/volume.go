package podman

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
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
	_, err = cli.VolumeDeleteLibpod(params)
	if err != nil {
		return err
	}
	return nil
}

func VolumesToMounts(c *container.Container) []*models.Mount {
	var mounts []*models.Mount
	for _, v := range c.Mounts {
		m := &models.Mount{
			ReadOnly:    !v.RW,
			Source:      v.Source,
			Target:      v.Destination,
			Destination: v.Destination,
			Type:        "volume",
			Options:     []string{"Z"},
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
			Options: []string{"Z"},
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
