package compat

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/skupperproject/skupper-libpod/v4/client/networks_compat"
	"github.com/skupperproject/skupper-libpod/v4/models"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/utils"
)

func (c *CompatClient) NetworkList() ([]*container.Network, error) {
	cli := networks_compat.New(c.RestClient, formats)
	params := networks_compat.NewNetworkListParams()
	res, err := cli.NetworkList(params)
	if err != nil {
		return nil, fmt.Errorf("error listing networks: %v", ToAPIError(err))
	}
	return ToNetworkInfoList(res.Payload), nil
}

func ToNetworkInfoList(networks []*models.NetworkResource) []*container.Network {
	var nets []*container.Network
	for _, net := range networks {
		nets = append(nets, ToNetworkInfo(net))
	}
	return nets
}

func ToNetworkInfo(network *models.NetworkResource) *container.Network {
	var ss []*container.Subnet

	if network.IPAM != nil {
		if len(network.IPAM.Config) > 0 {
			for _, s := range network.IPAM.Config {
				ss = append(ss, &container.Subnet{
					Subnet:  s.Subnet,
					Gateway: s.Gateway,
				})
			}
		}
	}

	n := &container.Network{
		ID:        network.ID,
		Name:      network.Name,
		Subnets:   ss,
		Driver:    network.Driver,
		IPV6:      network.EnableIPV6,
		DNS:       true,
		Internal:  network.Internal,
		Labels:    network.Labels,
		Options:   network.Options,
		CreatedAt: network.Created.String(),
	}

	return n
}

func (c *CompatClient) NetworkInspect(id string) (*container.Network, error) {
	cli := networks_compat.New(c.RestClient, formats)
	params := networks_compat.NewNetworkInspectParams()
	params.Name = id
	res, err := cli.NetworkInspect(params)
	if err != nil {
		return nil, fmt.Errorf("error inspecting network %s: %v", id, ToAPIError(err))
	}
	return ToNetworkInfo(res.Payload), nil
}

type networkCreateOK struct {
	// The ID of the created container
	// Required: true
	ID *string `json:"Id"`

	// Warnings encountered when creating the container
	// Required: true
	Warnings []string `json:"Warnings"`
}

type networkCreateResponseReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *networkCreateResponseReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200, 201:
		result := &networkCreateOK{}
		if err := consumer.Consume(response.Body(), result); err != nil && err != io.EOF {
			return result, err
		}
		return result, nil
	case 400, 403, 409:
		result := networks_compat.NewNetworkCreateBadRequest()
		result.Payload = new(networks_compat.NetworkCreateBadRequestBody)
		// response payload
		if err := consumer.Consume(response.Body(), result.Payload); err != nil && err != io.EOF {
			return result, err
		}
		return nil, result
	case 500:
		result := networks_compat.NewNetworkCreateInternalServerError()
		result.Payload = new(networks_compat.NetworkCreateInternalServerErrorBody)

		// response payload
		if err := consumer.Consume(response.Body(), result.Payload); err != nil && err != io.EOF {
			return result, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

func (c *CompatClient) NetworkCreate(network *container.Network) (*container.Network, error) {
	if network.Labels == nil {
		network.Labels = map[string]string{}
	}
	network.Labels["application"] = types.AppName
	params := networks_compat.NewNetworkCreateParams()
	params.Create = fromNetwork(network)

	op := &runtime.ClientOperation{
		ID:                 "NetworkCreate",
		Method:             "POST",
		PathPattern:        "/networks/create",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             &networkCreateResponseReader{},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}

	result, err := c.RestClient.Submit(op)
	if err != nil {
		return nil, fmt.Errorf("error creating network %s: %v", network.Name, ToAPIError(err))
	}
	switch v := result.(type) {
	case *networkCreateOK:
		network.ID = *v.ID
		return network, nil
	case *networks_compat.NetworkCreateBadRequest:
		return nil, fmt.Errorf("error creating network (bad request): %v", v.Payload.Message)
	case *networks_compat.NetworkCreateInternalServerError:
		return nil, fmt.Errorf("error creating network (internal server error): %v", v.Payload.Message)
	}
	return nil, fmt.Errorf("unable to parse network create response for %s: (%T) %v", network.Name, result, result)
}

func fromNetwork(network *container.Network) *models.NetworkCreateRequest {
	labels := map[string]string{
		types.PartOfLabel: types.AppName,
	}
	if network.Labels != nil {
		for k, v := range network.Labels {
			labels[k] = v
		}
	}
	n := &models.NetworkCreateRequest{
		Driver:     utils.DefaultStr(network.Driver, DefaultNetworkDriver),
		EnableIPV6: network.IPV6,
		IPAM: &models.IPAM{
			Config:  []*models.IPAMConfig{},
			Driver:  "default",
			Options: make(map[string]string),
		},
		Internal: network.Internal,
		Labels:   network.Labels,
		Name:     network.Name,
		Options:  network.Options,
	}
	for _, subnet := range network.Subnets {
		n.IPAM.Config = append(n.IPAM.Config, &models.IPAMConfig{
			Gateway: subnet.Gateway,
			Subnet:  subnet.Subnet,
		})
	}
	return n
}

func (c *CompatClient) NetworkRemove(id string) error {
	existing, err := c.NetworkInspect(id)
	if err != nil {
		return fmt.Errorf("network does not exist %s - %w", id, err)
	}
	if !container.IsOwnedBySkupper(existing.Labels) {
		return fmt.Errorf("network %s is not owned by Skupper", id)
	}
	cli := networks_compat.New(c.RestClient, formats)
	params := networks_compat.NewNetworkDeleteParams()
	params.Name = id
	_, err = cli.NetworkDelete(params)
	if err != nil {
		return fmt.Errorf("error removing network %s: %v", id, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) NetworkConnect(id, container string, aliases ...string) error {
	params := networks_compat.NewNetworkConnectParams()
	params.Name = id
	params.Create = &models.SwagCompatNetworkConnectRequest{
		Container: container,
		EndpointConfig: &models.EndpointSettings{
			Aliases: aliases,
		},
	}
	op := &runtime.ClientOperation{
		ID:                 "NetworkConnect",
		Method:             "POST",
		PathPattern:        "/networks/{name}/connect",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             &NetworkConnectReader{formats: formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	_, err := c.RestClient.Submit(op)
	if err != nil {
		return fmt.Errorf("error connecting %s to network %s: %v", container, id, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) NetworkDisconnect(id, container string) error {
	cli := networks_compat.New(c.RestClient, formats)
	params := networks_compat.NewNetworkDisconnectParams()
	params.Name = id
	params.Create = &models.SwagCompatNetworkDisconnectRequest{
		Container: container,
	}
	_, err := cli.NetworkDisconnect(params)
	if err != nil {
		return fmt.Errorf("error disconnecting %s from network %s: %v", container, id, ToAPIError(err))
	}
	return nil
}

type NetworkConnectReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *NetworkConnectReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := networks_compat.NewNetworkConnectOK()
		return result, nil
	case 400, 403, 409:
		result := networks_compat.NewNetworkConnectBadRequest()
		result.Payload = new(networks_compat.NetworkConnectBadRequestBody)
		if err := consumer.Consume(response.Body(), result.Payload); err != nil && err != io.EOF {
			return result, err
		}
		return nil, result
	case 500:
		result := networks_compat.NewNetworkConnectInternalServerError()
		result.Payload = new(networks_compat.NetworkConnectInternalServerErrorBody)
		if err := consumer.Consume(response.Body(), result.Payload); err != nil && err != io.EOF {
			return result, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}
