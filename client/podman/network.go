package podman

import (
	"fmt"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/networks"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
	"github.com/skupperproject/skupper/pkg/utils"
)

func (p *PodmanRestClient) NetworkList() ([]*container.Network, error) {
	cli := networks.New(p.RestClient, formats)
	params := networks.NewNetworkListLibpodParams()
	res, err := cli.NetworkListLibpod(params)
	if err != nil {
		return nil, fmt.Errorf("error listing networks: %v", err)
	}
	return ToNetworkInfoList(res.Payload), nil
}

func ToNetworkInfoList(networks []*models.Network) []*container.Network {
	var nets []*container.Network
	for _, net := range networks {
		nets = append(nets, ToNetworkInfo(net))
	}
	return nets
}

func ToNetworkInfo(network *models.Network) *container.Network {
	var ss []*container.Subnet

	for _, s := range network.Subnets {
		ss = append(ss, &container.Subnet{
			Subnet:  s.Subnet,
			Gateway: s.Gateway,
		})
	}

	n := &container.Network{
		ID:        network.ID,
		Name:      network.Name,
		Subnets:   ss,
		Driver:    network.Driver,
		DNS:       network.DNSEnabled,
		Internal:  network.Internal,
		Labels:    network.Labels,
		Options:   network.Options,
		CreatedAt: network.Created.String(),
	}

	return n
}

func (p *PodmanRestClient) NetworkInspect(id string) (*container.Network, error) {
	cli := networks.New(p.RestClient, formats)
	params := networks.NewNetworkInspectLibpodParams()
	params.Name = id
	res, err := cli.NetworkInspectLibpod(params)
	if err != nil {
		return nil, fmt.Errorf("error inspecting network %s: %v", id, err)
	}
	return ToNetworkInfo(res.Payload), nil
}

func (p *PodmanRestClient) NetworkCreate(network *container.Network) (*container.Network, error) {
	if network.Labels == nil {
		network.Labels = map[string]string{}
	}
	network.Labels["application"] = types.AppName
	cli := networks.New(p.RestClient, formats)
	params := networks.NewNetworkCreateLibpodParams()
	params.Create = fromNetwork(network)
	res, err := cli.NetworkCreateLibpod(params)
	if err != nil {
		return nil, fmt.Errorf("error creating network %s: %v", network.Name, err)
	}
	return ToNetworkInfo(res.Payload), nil
}

func fromNetwork(network *container.Network) *models.SwagNetworkCreateLibpod {
	labels := map[string]string{
		types.PartOfLabel: types.AppName,
	}
	if network.Labels != nil {
		for k, v := range network.Labels {
			labels[k] = v
		}
	}
	n := &models.SwagNetworkCreateLibpod{
		Created:     strfmt.DateTime(time.Now()),
		DNSEnabled:  network.DNS,
		Driver:      utils.DefaultStr(network.Driver, DefaultNetworkDriver),
		IPV6Enabled: true,
		Internal:    network.Internal,
		Labels:      labels,
		Name:        network.Name,
		Options:     network.Options,
		Subnets:     fromSubnets(network.Subnets),
	}
	return n
}

func fromSubnets(subnets []*container.Subnet) []*models.Subnet {
	var ss []*models.Subnet
	for _, subnet := range subnets {
		ss = append(ss, &models.Subnet{
			Gateway: subnet.Gateway,
			Subnet:  subnet.Subnet,
		})
	}
	return ss
}

func (p *PodmanRestClient) NetworkRemove(id string) error {
	cli := networks.New(p.RestClient, formats)
	params := networks.NewNetworkDeleteLibpodParams()
	params.Force = boolTrue()
	params.Name = id
	_, err := cli.NetworkDeleteLibpod(params)
	if err != nil {
		return fmt.Errorf("error removing network %s: %v", id, err)
	}
	return nil
}

func (p *PodmanRestClient) NetworkConnect(id, container string, aliases ...string) error {
	cli := networks.New(p.RestClient, formats)
	params := networks.NewNetworkConnectLibpodParams()
	params.Name = id
	params.Create = &models.SwagNetworkConnectRequest{
		Aliases:   aliases,
		Container: container,
	}
	_, err := cli.NetworkConnectLibpod(params)
	if err != nil {
		return fmt.Errorf("error connecting %s to network %s: %v", container, id, err)
	}
	return nil
}

func (p *PodmanRestClient) NetworkDisconnect(id, container string) error {
	cli := networks.New(p.RestClient, formats)
	params := networks.NewNetworkDisconnectLibpodParams()
	params.Name = id
	params.Create = &models.SwagCompatNetworkDisconnectRequest{
		Container: container,
		Force:     true,
	}
	_, err := cli.NetworkDisconnectLibpod(params)
	if err != nil {
		return fmt.Errorf("error disconnecting %s from network %s: %v", container, id, err)
	}
	return nil
}
