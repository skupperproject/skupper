package podman

import (
	"context"
	"fmt"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	runtimeclient "github.com/go-openapi/runtime/client"
	"github.com/skupperproject/skupper/api/types"
	"github.com/fgiorgetti/skupper-libpod/client/containers"
	"github.com/fgiorgetti/skupper-libpod/client/exec"
	"github.com/fgiorgetti/skupper-libpod/models"
	"github.com/skupperproject/skupper/pkg/container"
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
	curUser, err := user.Current()
	var containerUser string
	var idMappings *models.IDMappingOptions
	var userNs *models.Namespace

	if err == nil {
		// this is based on:
		// https://www.redhat.com/sysadmin/debug-rootless-podman-mounted-volumes
		// idMappings is mandatory and is set to the same value podman sets it
		// when using --userns=keep-id
		containerUser = curUser.Uid + ":" + curUser.Gid
		idMappings = &models.IDMappingOptions{
			HostGIDMapping: false,
			HostUIDMapping: false,
		}
		if curUser.Uid != "0" {
			userNs = &models.Namespace{Nsmode: models.NamespaceMode("keep-id")}
		}
	}
	spec := &models.SpecGenerator{
		Annotations:   c.Annotations,
		CNINetworks:   c.NetworkNames(),
		Command:       c.Command,
		Entrypoint:    c.EntryPoint,
		Env:           c.Env,
		Image:         c.Image,
		Labels:        c.Labels,
		Mounts:        FilesToMounts(c),
		Volumes:       VolumesToNamedVolumes(c),
		Name:          c.Name,
		Pod:           c.Pod,
		PortMappings:  ToPortmappings(c),
		RestartPolicy: c.RestartPolicy,
		User:          containerUser,
		Userns:        userNs,
		Idmappings:    idMappings,
	}
	// resource limits set like using --cpus and --memory through CLI
	if c.MaxCpus > 0 || c.MaxMemoryBytes > 0 {
		spec.ResourceLimits = &models.LinuxResources{}
	}
	if c.MaxCpus > 0 {
		spec.ResourceLimits.CPU = &models.LinuxCPU{
			Quota:  int64(c.MaxCpus * 100000),
			Period: 100000,
		}
	}
	if c.MaxMemoryBytes > 0 {
		spec.ResourceLimits.Memory = &models.LinuxMemory{
			Limit: c.MaxMemoryBytes,
		}
	}

	if c.Annotations != nil && c.Annotations["io.podman.annotations.label"] == "disable" {
		spec.SelinuxOpts = append(spec.SelinuxOpts, "disable")
	}

	// Network info
	spec.Networks = map[string]models.PerNetworkOptions{}
	spec.Netns = &models.Namespace{
		Nsmode: "bridge",
	}
	for networkName, network := range c.Networks {
		// aliases must be populated when dns is enabled for the network
		if len(network.Aliases) == 0 {
			network.Aliases = append(network.Aliases, c.Name)
		}
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

// ContainerUpdate replaces the container (by name) with an identical copy
// with an applied Customization (required).
// To achieve that, it follows this procedure:
// - Create a new container
// - Stop current container
// - Rename current container
// - Rename the new container (replacing the original one)
// - Starts the new container
// - Removes the original container
//
// In case of failures during this process, the original container is restored.
func (p *PodmanRestClient) ContainerUpdate(name string, fn func(newContainer *container.Container)) (*container.Container, error) {
	if fn == nil {
		return nil, fmt.Errorf("at least one customization is needed")
	}

	datetime := time.Now().Format("20060102150405")
	c, err := p.ContainerInspect(name)
	if err != nil {
		return nil, err
	}

	newContainerName := fmt.Sprintf("%s-new-%s", c.Name, datetime)
	cc := &container.Container{
		Name:           newContainerName,
		Image:          c.Image,
		Env:            c.Env,
		Labels:         c.Labels,
		Annotations:    c.Annotations,
		Networks:       c.Networks,
		Mounts:         c.Mounts,
		FileMounts:     c.FileMounts,
		Ports:          c.Ports,
		EntryPoint:     c.EntryPoint,
		Command:        c.Command,
		RestartPolicy:  c.RestartPolicy,
		MaxCpus:        c.MaxCpus,
		MaxMemoryBytes: c.MaxMemoryBytes,
	}

	// apply new container customization
	fn(cc)
	if cc.Name != newContainerName {
		return nil, fmt.Errorf("container name cannot be changed")
	}

	// creating a new container
	err = p.ContainerCreate(cc)
	if err != nil {
		return cc, err
	}

	// rollback this one in case of failures below
	defer func() {
		if err != nil {
			_ = p.ContainerStop(cc.Name)
			_ = p.ContainerRemove(cc.Name)
		}
	}()

	// stopping current container
	err = p.ContainerStop(name)
	if err != nil {
		return cc, err
	}

	// restarting current container
	defer func() {
		if err != nil {
			_ = p.ContainerStart(name)
		}
	}()

	// renaming current container to a backup name
	backupName := fmt.Sprintf("%s-%s", name, datetime)
	err = p.ContainerRename(name, backupName)
	if err != nil {
		return cc, err
	}

	// restoring original container
	defer func() {
		if err != nil {
			_ = p.ContainerRename(backupName, name)
		}
	}()

	// renaming new container to current name
	err = p.ContainerRename(cc.Name, name)
	if err != nil {
		return cc, err
	}

	defer func() {
		if err != nil {
			_ = p.ContainerRename(name, newContainerName)
		}
	}()

	// starting new container
	err = p.ContainerStart(name)
	if err != nil {
		return cc, err
	} else {
		if removeErr := p.ContainerRemove(backupName); removeErr != nil {
			fmt.Printf("Unable to remove backup container: %s - %s", backupName, err)
			fmt.Println()
		}
	}

	return cc, nil
}

// ContainerUpdateImage updates the image used by the given container (name).
func (p *PodmanRestClient) ContainerUpdateImage(ctx context.Context, name string, newImage string) (*container.Container, error) {
	err := p.ImagePull(ctx, newImage)
	if err != nil {
		return nil, fmt.Errorf("error pulling image %q: %s", newImage, err)
	}
	return p.ContainerUpdate(name, func(newContainer *container.Container) {
		newContainer.Image = newImage
	})
}

func (p *PodmanRestClient) ContainerRename(currentName, newName string) error {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerRenameLibpodParams()
	params.PathName = currentName
	params.QueryName = newName
	_, err := cli.ContainerRenameLibpod(params)
	if err != nil {
		return fmt.Errorf("error renaming container %s to %s: %v", currentName, newName, err)
	}
	return nil
}

func (p *PodmanRestClient) ContainerRemove(name string) error {
	cli := containers.New(p.RestClient, formats)
	params := containers.NewContainerDeleteLibpodParams()
	params.Name = name
	params.Force = boolTrue()
	params.Ignore = boolTrue()
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
		_, notRunning := err.(*containers.ContainerStopLibpodNotModified)
		if !notRunning {
			return fmt.Errorf("error stopping container %s: %v", name, err)
		}
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
		return "", fmt.Errorf("error executing command on %s: %v", id, err)
	}
	resp, ok := result.(*models.IDResponse)
	if !ok {
		return "", fmt.Errorf("error parsing execution id")
	}

	// Starting the exec
	startParams := exec.NewExecStartLibpodParams()
	startParams.ID = resp.ID

	reader := &multiplexedBodyReader{}
	startOp := &runtime.ClientOperation{
		ID:                 "ExecStartLibpod",
		Method:             "POST",
		PathPattern:        "/libpod/exec/{id}/start",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             startParams,
		Reader:             reader,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}

	restClient, ok := p.RestClient.(*runtimeclient.Runtime)
	if ok {
		restClient.Consumers["*/*"] = reader
	}
	result, err = p.RestClient.Submit(startOp)
	if err != nil {
		return "", fmt.Errorf("error starting execution: %v", err)
	}
	// stdout and stderr are also available under reader.Stdout() and reader.Stderr()
	out, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("error parsing response")
	}
	return out, nil
}

func FromListContainer(c models.ListContainer) *container.Container {
	ct := &container.Container{
		ID:       c.ID,
		Name:     c.Names[0],
		Pod:      c.Pod,
		Image:    c.Image,
		Labels:   c.Labels,
		Command:  c.Command,
		Running:  !c.Exited,
		ExitCode: int(c.ExitCode),
	}
	// when listing containers dates are returned in unix time
	createdAt, _ := strconv.ParseInt(c.CreatedAt, 10, 64)
	ct.CreatedAt = time.Unix(createdAt, 0)
	ct.StartedAt = time.Unix(c.StartedAt, 0)
	ct.ExitedAt = time.Unix(c.ExitedAt, 0)

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
		CreatedAt:    time.Time(c.Created),
		Pod:          c.Pod,
	}
	ct.Networks = map[string]container.ContainerNetworkInfo{}
	ct.Labels = map[string]string{}
	ct.Annotations = map[string]string{}
	ct.Env = map[string]string{}

	// Volume mounts
	for _, m := range c.Mounts {
		switch m.Type {
		case "volume":
			ct.Mounts = append(ct.Mounts, container.Volume{
				Name:        m.Name,
				Source:      m.Source,
				Destination: m.Destination,
				Mode:        m.Mode,
				RW:          m.RW,
			})
		case "bind":
			ct.FileMounts = append(ct.FileMounts, container.FileMount{
				Source:      m.Source,
				Destination: m.Destination,
				Options:     m.Options,
			})
		}
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
		if hostConfig.CPUQuota > 0 {
			ct.MaxCpus = int(hostConfig.CPUQuota / 100000)
		}
		if hostConfig.Memory > 0 {
			ct.MaxMemoryBytes = hostConfig.Memory
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
		ct.StartedAt = time.Time(c.State.StartedAt)
		ct.ExitedAt = time.Time(c.State.FinishedAt)
		ct.ExitCode = int(c.State.ExitCode)
	}

	return ct
}

func (p *PodmanRestClient) ContainerLogs(id string) (string, error) {
	params := containers.NewContainerLogsLibpodParams()
	params.Name = id
	params.Stdout = boolTrue()
	params.Stderr = boolTrue()
	reader := &responseReaderOctetStreamBody{}
	op := &runtime.ClientOperation{
		ID:                 "ContainerLogsLibpod",
		Method:             "GET",
		PathPattern:        "/libpod/containers/{name}/logs",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             reader,
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
