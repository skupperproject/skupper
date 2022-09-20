package podman

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/containers"
	"github.com/skupperproject/skupper/client/generated/libpod/client/exec"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
)

func (p *PodmanRestClient) ContainerList() ([]*container.Container, error) {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerListLibpodParams()
	params.All = boolTrue()
	list, err := cli.ContainerListLibpod(params)
	if err != nil {
		return nil, fmt.Errorf("error listing containers: %v", err)
	}
	cts := []*container.Container{}
	for _, c := range list.Payload {
		if c == nil {
			continue
		}
		cts = append(cts, FromListContainer(*c))
	}
	return cts, nil
}

func (p *PodmanRestClient) ContainerInspect(id string) (*container.Container, error) {
	cli := containers.New(p.RestClient, formats)
	param := containers.NewContainerInspectLibpodParams()
	param.Name = id
	res, err := cli.ContainerInspectLibpod(param)
	if err != nil {
		return nil, fmt.Errorf("error inspecting container '%s': %v", id, err)
	}
	return FromInspectContainer(*res.Payload), nil
}

func ToSpecGenerator(c *container.Container) *models.SpecGenerator {
	spec := &models.SpecGenerator{
		Annotations: c.Annotations,
		CNINetworks: c.NetworkNames(),
		Command:     c.Command,
		Entrypoint:  c.EntryPoint,
		Env:         c.Env,
		Image:       c.Image,
		Labels:      c.Labels,
		// Mounts:      VolumesToMounts(c),
		Volumes:       VolumesToNamedVolumes(c),
		Name:          c.Name,
		Pod:           c.Pod,
		PortMappings:  ToPortmappings(c),
		RestartPolicy: c.RestartPolicy,
	}

	// Network info
	spec.Networks = map[string]models.PerNetworkOptions{}
	spec.Netns = &models.Namespace{
		Nsmode: "bridge",
	}
	for networkName, network := range c.Networks {
		spec.Networks[networkName] = models.PerNetworkOptions{
			Aliases: network.Aliases,
		}
	}
	return spec
}

func ToPortmappings(c *container.Container) []*models.PortMapping {
	var mapping []*models.PortMapping
	for _, port := range c.Ports {
		target, _ := strconv.Atoi(port.Target)
		host, _ := strconv.Atoi(port.Host)

		mapping = append(mapping, &models.PortMapping{
			ContainerPort: uint16(target),
			HostIP:        port.HostIP,
			HostPort:      uint16(host),
			Protocol:      port.Protocol,
		})
	}
	return mapping
}

func (p *PodmanRestClient) ContainerCreate(container *container.Container) error {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerCreateLibpodParams()
	if container.Labels == nil {
		container.Labels = map[string]string{}
	}
	container.Labels["application"] = types.AppName
	params.Create = ToSpecGenerator(container)
	_, err := cli.ContainerCreateLibpod(params)
	if err != nil {
		return fmt.Errorf("error creating container %s: %v", container.Name, err)
	}
	return nil
}

func (p *PodmanRestClient) ContainerRemove(name string) error {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerDeleteLibpodParams()
	params.Name = name
	params.Force = boolTrue()
	_, _, err := cli.ContainerDeleteLibpod(params)
	if err != nil {
		return fmt.Errorf("error deleting container %s: %v", name, err)
	}
	return nil
}

func (p *PodmanRestClient) ContainerStart(name string) error {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerStartLibpodParams()
	params.Name = name
	_, err := cli.ContainerStartLibpod(params)
	if err != nil {
		return fmt.Errorf("error starting container %s: %v", name, err)
	}
	return nil
}

func (p *PodmanRestClient) ContainerStop(name string) error {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerStopLibpodParams()
	params.Name = name
	params.Ignore = boolTrue()
	_, err := cli.ContainerStopLibpod(params)
	if err != nil {
		return fmt.Errorf("error stopping container %s: %v", name, err)
	}
	return nil
}

func (p *PodmanRestClient) ContainerRestart(name string) error {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerRestartLibpodParams()
	params.Name = name
	_, err := cli.ContainerRestartLibpod(params)
	if err != nil {
		return fmt.Errorf("error restarting container %s: %v", name, err)
	}
	return nil
}

func (p *PodmanRestClient) ContainerExec(id string, command []string) (string, error) {
	params := exec.NewContainerExecLibpodParams()
	params.Name = id
	params.Control = exec.ContainerExecLibpodBody{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          command,
	}
	// Creating the exec
	execOp := &runtime.ClientOperation{
		ID:                 "ContainerExecLibpod",
		Method:             "POST",
		PathPattern:        "/libpod/containers/{name}/exec",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             &responseReaderID{},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	result, err := p.RestClient.Submit(execOp)
	if err != nil {
		return "", fmt.Errorf("error executing command: %v", err)
	}
	resp, ok := result.(*models.IDResponse)
	if !ok {
		return "", fmt.Errorf("error parsing execution id")
	}

	// Starting the exec
	startParams := exec.NewExecStartLibpodParams()
	startParams.ID = resp.ID

	startOp := &runtime.ClientOperation{
		ID:                 "ExecStartLibpod",
		Method:             "POST",
		PathPattern:        "/libpod/exec/{id}/start",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             startParams,
		Reader:             &responseReaderBody{},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}

	p.RestClient.Consumers["*/*"] = &responseReaderBody{}
	result, err = p.RestClient.Submit(startOp)
	if err != nil {
		return "", fmt.Errorf("error starting execution: %v", err)
	}
	out, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("error parsing response")
	}
	return out, nil
}

/*
	ContainerExec(id string, command []string) (string, string, error)
	ContainerLogs(id string) (string, error)
*/

func FromListContainer(c models.ListContainer) *container.Container {
	ct := &container.Container{
		ID:        c.ID,
		Name:      c.Names[0],
		Pod:       c.Pod,
		Image:     c.Image,
		Labels:    c.Labels,
		Command:   c.Command,
		Running:   !c.Exited,
		CreatedAt: fmt.Sprint(c.CreatedAt),
		StartedAt: fmt.Sprint(c.StartedAt),
		ExitedAt:  fmt.Sprint(c.ExitedAt),
		ExitCode:  int(c.ExitCode),
	}
	ct.Networks = map[string]container.ContainerNetworkInfo{}
	ct.Env = map[string]string{}

	// base network info
	for _, n := range c.Networks {
		network := container.ContainerNetworkInfo{ID: n}
		ct.Networks[n] = network
	}
	// base mount info
	for _, m := range c.Mounts {
		v := container.Volume{Destination: m}
		ct.Mounts = append(ct.Mounts, v)
	}
	// port mapping
	for _, port := range c.Ports {
		ct.Ports = append(ct.Ports, container.Port{
			Host:     fmt.Sprint(port.HostPort),
			HostIP:   port.HostIP,
			Target:   fmt.Sprint(port.ContainerPort),
			Protocol: port.Protocol,
		})
	}
	return ct
}

func FromInspectContainer(c containers.ContainerInspectLibpodOKBody) *container.Container {
	ct := &container.Container{
		ID:           c.ID,
		Name:         c.Name,
		RestartCount: int(c.RestartCount),
		CreatedAt:    c.Created.String(),
		Pod:          c.Pod,
	}
	ct.Networks = map[string]container.ContainerNetworkInfo{}
	ct.Labels = map[string]string{}
	ct.Annotations = map[string]string{}
	ct.Env = map[string]string{}

	// Volume mounts
	for _, m := range c.Mounts {
		volume := container.Volume{
			Name:        m.Name,
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
		}
		ct.Mounts = append(ct.Mounts, volume)
	}

	// Container config
	if c.Config != nil {
		config := c.Config
		ct.Image = config.Image
		ct.FromEnv(config.Env)
		ct.Labels = config.Labels
		ct.Annotations = config.Annotations
		if config.Entrypoint != "" {
			ct.EntryPoint = []string{config.Entrypoint}
		}
		ct.Command = config.Cmd
	}

	// HostConfig
	if c.HostConfig != nil {
		hostConfig := c.HostConfig
		if hostConfig.RestartPolicy != nil {
			ct.RestartPolicy = hostConfig.RestartPolicy.Name
		}
	}

	// Network info
	if c.NetworkSettings != nil {
		// Addressing info
		for k, v := range c.NetworkSettings.Networks {
			netInfo := container.ContainerNetworkInfo{
				ID:          v.NetworkID,
				IPAddress:   v.IPAddress,
				IPPrefixLen: int(v.IPPrefixLen),
				MacAddress:  v.MacAddress,
				Gateway:     v.Gateway,
				Aliases:     v.Aliases,
			}
			ct.Networks[k] = netInfo
		}

		// Port mapping
		for portProto, ports := range c.NetworkSettings.Ports {
			portProtoS := strings.Split(portProto, "/")
			protocol := "tcp"
			if len(portProtoS) > 1 {
				protocol = portProtoS[1]
			}
			targetPort := portProtoS[0]
			for _, portInfo := range ports {
				p := container.Port{
					Host:     portInfo.HostPort,
					HostIP:   portInfo.HostIP,
					Target:   targetPort,
					Protocol: protocol,
				}
				ct.Ports = append(ct.Ports, p)
			}
		}
	}

	// State info
	if c.State != nil {
		ct.Running = c.State.Running
		ct.StartedAt = c.State.StartedAt.String()
		ct.ExitedAt = c.State.FinishedAt.String()
		ct.ExitCode = int(c.State.ExitCode)
	}

	return ct
}

func (p *PodmanRestClient) ContainerLogs(id string) (string, error) {
	params := containers.NewContainerLogsLibpodParams()
	params.Name = id
	params.Stdout = boolTrue()
	params.Stderr = boolTrue()
	op := &runtime.ClientOperation{
		ID:                 "ContainerLogsLibpod",
		Method:             "GET",
		PathPattern:        "/libpod/containers/{name}/logs",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             &responseReaderBody{},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	result, err := p.RestClient.Submit(op)
	if err != nil {
		return "", fmt.Errorf("error retrieving logs from container %s: %v", id, err)
	}
	logs := result.(string)
	return logs, nil
}
