package compat

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes_compat"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
	"github.com/skupperproject/skupper/pkg/container"
)

func (c *CompatClient) VolumeCreate(volume *container.Volume) (*container.Volume, error) {
	if volume.Labels == nil {
		volume.Labels = map[string]string{}
	}
	volume.Labels["application"] = types.AppName
	cli := volumes_compat.New(c.RestClient, formats)
	params := volumes_compat.NewVolumeCreateParams()
	params.Create = ToVolumeCreate(volume)
	resp, err := cli.VolumeCreate(params)
	if err != nil {
		return nil, fmt.Errorf("error creating volume %s: %w", volume.Name, ToAPIError(err))
	}
	return FromCreatedVolume(resp), nil
}

func ToVolumeCreate(volume *container.Volume) *models.DockerVolumeCreate {
	return &models.DockerVolumeCreate{
		VolumeCreateBody: models.VolumeCreateBody{
			Name:   stringP(volume.Name),
			Labels: volume.Labels,
		},
	}
}

func FromCreatedVolume(created *volumes_compat.VolumeCreateCreated) *container.Volume {
	return &container.Volume{
		Name:   *created.Payload.Name,
		Source: *created.Payload.Mountpoint,
		Labels: created.Payload.Labels,
	}
}

func (c *CompatClient) VolumeInspect(id string) (*container.Volume, error) {
	cli := volumes_compat.New(c.RestClient, formats)
	params := volumes_compat.NewVolumeInspectParams()
	params.Name = id
	resp, err := cli.VolumeInspect(params)
	if err != nil {
		return nil, fmt.Errorf("error inspecting volume %s: %w", id, ToAPIError(err))
	}
	return FromInspectVolume(resp), nil
}

func FromInspectVolume(volumeInspect *volumes_compat.VolumeInspectOK) *container.Volume {
	return &container.Volume{
		Name:   *volumeInspect.Payload.Name,
		Source: *volumeInspect.Payload.Mountpoint,
		Labels: volumeInspect.Payload.Labels,
	}
}

func (c *CompatClient) VolumeRemove(id string) error {
	existing, err := c.VolumeInspect(id)
	if err != nil {
		return err
	}
	if !container.IsOwnedBySkupper(existing.Labels) {
		return fmt.Errorf("volume %s is not owned by skupper", id)
	}
	cli := volumes_compat.New(c.RestClient, formats)
	params := volumes_compat.NewVolumeDeleteParams()
	params.Name = id
	_, err = cli.VolumeDelete(params)
	if err != nil {
		return fmt.Errorf("error removing volume %s: %w", id, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) VolumeList() ([]*container.Volume, error) {
	params := volumes_compat.NewVolumeListParams()
	op := &runtime.ClientOperation{
		ID:                 "VolumeList",
		Method:             "GET",
		PathPattern:        "/volumes",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             &VolumeListReader{formats: formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	result, err := c.RestClient.Submit(op)
	if err != nil {
		return nil, ToAPIError(err)
	}
	success, ok := result.(*models.VolumeListOKBody)
	if ok {
		return FromVolumeList(success), nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	return nil, fmt.Errorf("unexpected success response for VolumeList: API contract not enforced by server. Client expected to get an error, but got: %T", result)
}

func FromVolumeList(volumesList *models.VolumeListOKBody) []*container.Volume {
	var volumes []*container.Volume
	for _, vol := range volumesList.Volumes {
		volumes = append(volumes, &container.Volume{
			Name:   *vol.Name,
			Source: *vol.Mountpoint,
			Labels: vol.Labels,
		})
	}
	return volumes
}

type VolumeListReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *VolumeListReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := &models.VolumeListOKBody{}
		if err := consumer.Consume(response.Body(), result); err != nil && err != io.EOF {
			return nil, err
		}
		return result, nil
	case 500:
		result := volumes_compat.NewVolumeListInternalServerError()
		result.Payload = new(volumes_compat.VolumeListInternalServerErrorBody)
		if err := consumer.Consume(response.Body(), result.Payload); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}
