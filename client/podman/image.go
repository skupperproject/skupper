package podman

import (
	"fmt"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/images"
)

func (p *PodmanRestClient) ImageList() ([]*container.Image, error) {
	cli := images.New(p.RestClient, formats)
	param := images.NewImageListLibpodParams()
	param.All = boolTrue()
	res, err := cli.ImageListLibpod(param)
	if err != nil {
		return nil, fmt.Errorf("error listing images: %v", err)
	}
	var imgs []*container.Image
	for _, img := range res.Payload {
		for _, imgName := range img.RepoTags {
			// for _, imgName := range img.Names {
			imgs = append(imgs, &container.Image{
				Id:         img.ID,
				Repository: imgName,
				Created:    fmt.Sprint(img.Created),
			})
		}
	}
	return imgs, nil
}

func (p *PodmanRestClient) ImageInspect(id string) (*container.Image, error) {
	cli := images.New(p.RestClient, formats)
	param := images.NewImageInspectLibpodParams()
	param.Name = id
	res, err := cli.ImageInspectLibpod(param)
	if err != nil {
		return nil, fmt.Errorf("error inspecting image %s: %v", id, err)
	}
	img := &container.Image{
		Id:         res.Payload.ID,
		Repository: res.Payload.RepoTags[0],
		Digest:     string(res.Payload.Digest),
		Created:    res.Payload.Created.String(),
	}
	if !strings.HasPrefix(img.Id, id) {
		for _, name := range res.Payload.RepoTags {
			if strings.Contains(name, id) {
				img.Repository = name
				break
			}
		}
	}

	return img, nil
}

func (p *PodmanRestClient) ImagePull(id string) error {
	params := images.NewImagePullLibpodParams()
	params.Reference = &id
	params.TLSVerify = new(bool)
	params.AllTags = new(bool)
	params.Quiet = new(bool)
	params.Arch = stringP("")
	params.OS = stringP("")
	params.Variant = stringP("")
	params.Policy = stringP("always")

	// Need to do that as the default response reader is being closed too soon
	op := &runtime.ClientOperation{
		ID:                 "ImagePullLibpod",
		Method:             "POST",
		PathPattern:        "/libpod/images/pull",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             &responseReaderBody{},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	_, err := p.RestClient.Submit(op)
	if err != nil {
		return fmt.Errorf("error pulling image %s: %v", id, err)
	}
	return nil
}
