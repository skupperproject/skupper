package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	runtimeclient "github.com/go-openapi/runtime/client"
	"github.com/skupperproject/skupper-libpod/v4/client/containers_compat"
	"github.com/skupperproject/skupper-libpod/v4/client/exec_compat"
	"github.com/skupperproject/skupper-libpod/v4/models"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/container"
)

func (c *CompatClient) ContainerList() ([]*container.Container, error) {
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerListParams()
	params.All = boolTrue()
	list, err := cli.ContainerList(params)
	if err != nil {
		return nil, fmt.Errorf("error listing containers: %v", ToAPIError(err))
	}
	var cts []*container.Container
	for _, c := range asInterfaceSlice(list.Payload) {
		if c == nil {
			continue
		}
		cMap := asStringInterfaceMap(c)
		ct := &container.Container{
			ID:         cMap["Id"].(string),
			Name:       asInterfaceSlice(cMap["Names"])[0].(string)[1:],
			Image:      cMap["Image"].(string),
			Labels:     asStringStringMap(cMap["Labels"]),
			Networks:   make(map[string]container.ContainerNetworkInfo),
			Mounts:     make([]container.Volume, 0),
			FileMounts: make([]container.FileMount, 0),
			Ports:      make([]container.Port, 0),
			Command:    []string{cMap["Command"].(string)},
			Running:    cMap["State"].(string) == "running",
			CreatedAt:  time.Unix(jsonNumberAsInt(cMap["Created"]), 0),
		}
		// Network info
		for k, v := range asStringInterfaceMap(cMap["NetworkSettings"]) {
			if k != "Networks" || v == nil {
				continue
			}
			for networkName, networkInfo := range asStringInterfaceMap(v) {
				networkInfoMap := asStringInterfaceMap(networkInfo)
				ipPrefixLen := networkInfoMap["IPPrefixLen"].(json.Number)
				ipPrefixLenInt64, _ := ipPrefixLen.Int64()
				ct.Networks[networkName] = container.ContainerNetworkInfo{
					ID:          networkInfoMap["NetworkID"].(string),
					IPAddress:   networkInfoMap["IPAddress"].(string),
					IPPrefixLen: int(ipPrefixLenInt64),
					MacAddress:  networkInfoMap["MacAddress"].(string),
					Gateway:     networkInfoMap["Gateway"].(string),
					Aliases:     asSlice[string](networkInfoMap["Aliases"]),
				}
			}
		}
		// Mounts and FileMounts
		for _, v := range asInterfaceSlice(cMap["Mounts"]) {
			mountsMap := asStringInterfaceMap(v)
			if mountType, ok := mountsMap["Type"]; ok && mountType == "bind" {
				// FileMount
				ct.FileMounts = append(ct.FileMounts, container.FileMount{
					Source:      mountsMap["Source"].(string),
					Destination: mountsMap["Destination"].(string),
				})
			} else {
				// Mount (volume)
				ct.Mounts = append(ct.Mounts, container.Volume{
					Name:        mountsMap["Name"].(string),
					Source:      mountsMap["Source"].(string),
					Destination: mountsMap["Destination"].(string),
					Mode:        mountsMap["Mode"].(string),
					RW:          mountsMap["RW"].(bool),
				})
			}
		}
		// Ports
		for _, v := range asInterfaceSlice(cMap["Ports"]) {
			portsMap := asStringInterfaceMap(v)
			port := container.Port{
				Host:     strconv.FormatInt(jsonNumberAsInt(portsMap["PublicPort"]), 10),
				Target:   strconv.FormatInt(jsonNumberAsInt(portsMap["PrivatePort"]), 10),
				Protocol: portsMap["Type"].(string),
			}
			if ip, ok := portsMap["IP"]; ok && ip != nil {
				port.HostIP = ip.(string)
			}
			ct.Ports = append(ct.Ports, port)
		}
		cts = append(cts, ct)
	}
	return cts, nil
}

func (c *CompatClient) ContainerInspect(id string) (*container.Container, error) {
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerInspectParams()
	params.Name = id
	inspect, err := cli.ContainerInspect(params)
	if err != nil {
		return nil, fmt.Errorf("error inspecting container %q: %v", id, ToAPIError(err))
	}
	return FromInspectContainer(inspect.Payload), nil
}

func FromInspectContainer(c *containers_compat.ContainerInspectOKBody) *container.Container {
	containerName := c.Name
	if containerName[0] == '/' {
		containerName = containerName[1:]
	}
	ct := &container.Container{
		ID:           c.ID,
		Name:         containerName,
		RestartCount: int(c.RestartCount),
	}
	created, _ := time.Parse(time.RFC3339, c.Created)
	ct.CreatedAt = created
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
				Propagation: string(m.Propagation),
				RW:          m.RW,
			})
		}
	}

	// Container config
	if c.Config != nil {
		config := c.Config
		ct.Image = config.Image
		ct.FromEnv(config.Env)
		ct.Labels = config.Labels
		if len(config.Entrypoint) > 0 {
			ct.EntryPoint = config.Entrypoint
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
		startedAt, _ := time.Parse(time.RFC3339, c.State.StartedAt)
		exitedAt, _ := time.Parse(time.RFC3339, c.State.FinishedAt)
		ct.StartedAt = startedAt
		ct.ExitedAt = exitedAt
		ct.ExitCode = int(c.State.ExitCode)
	}

	return ct
}

func (c *CompatClient) ContainerCreate(container *container.Container) error {
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerCreateParams()
	if container.Labels == nil {
		container.Labels = map[string]string{}
	}
	container.Labels["application"] = types.AppName
	params.Name = &container.Name
	params.Body = c.ToSpecGenerator(container)
	_, err := cli.ContainerCreate(params)
	if err != nil {
		return fmt.Errorf("error creating container %s: %v", container.Name, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) ToSpecGenerator(newContainer *container.Container) *models.CreateContainerConfig {
	curUser, err := user.Current()
	var userNs models.UsernsMode

	if err == nil {
		if curUser.Uid != "0" {
			userNs = "keep-id"
		}
	}
	spec := &models.CreateContainerConfig{
		HostConfig: &models.HostConfig{
			// Populate Binds if you need to mount volumes (format: source:dest:options)
			Binds:        nil,
			Mounts:       make([]*models.Mount, 0),
			NetworkMode:  "host",
			OomScoreAdj:  100, // this is the default value set by podman
			PortBindings: models.PortMap{},
			RestartPolicy: &models.RestartPolicy{
				Name: newContainer.RestartPolicy,
			},
			UsernsMode: userNs,
		},
		Cmd:        newContainer.Command,
		Entrypoint: newContainer.EntryPoint,
		Env:        mapToSlice(newContainer.Env),
		Image:      newContainer.Image,
		Labels:     newContainer.Labels,
		Name:       newContainer.Name,
	}
	spec.User = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if c.engine == "docker" {
		spec.User = "0:0"
		dockerGroup, err := user.LookupGroup("docker")
		if err == nil {
			spec.User = fmt.Sprintf("0:%s", dockerGroup.Gid)
		}
	}
	// Resource settings
	if newContainer.MaxCpus > 0 {
		spec.HostConfig.CPUCount = int64(newContainer.MaxCpus)
		spec.HostConfig.CPUPeriod = 100000
		spec.HostConfig.CPUQuota = int64(newContainer.MaxCpus * 100000)
	}
	if newContainer.MaxMemoryBytes > 0 {
		spec.HostConfig.Memory = newContainer.MaxMemoryBytes
	}
	// Network info
	if len(newContainer.Networks) > 0 {
		spec.HostConfig.NetworkMode = "bridge"
		spec.NetworkingConfig = &models.NetworkingConfig{
			EndpointsConfig: map[string]models.EndpointSettings{},
		}
		for network, networkInfo := range newContainer.Networks {
			spec.NetworkingConfig.EndpointsConfig[network] = models.EndpointSettings{
				Aliases:     networkInfo.Aliases,
				Gateway:     networkInfo.Gateway,
				IPAddress:   networkInfo.IPAddress,
				IPPrefixLen: int64(networkInfo.IPPrefixLen),
				MacAddress:  networkInfo.MacAddress,
				NetworkID:   networkInfo.ID,
			}
		}
		spec.HostConfig.PortBindings = models.PortMap{}
		for _, port := range newContainer.Ports {
			portKey := fmt.Sprintf("%s/tcp", port.Target)
			spec.HostConfig.PortBindings[portKey] = []models.PortBinding{
				{
					HostIP:   port.HostIP,
					HostPort: port.Host,
				},
			}
		}
	}
	// File mounts
	for _, mount := range newContainer.FileMounts {
		if mount.Source == "" || mount.Destination == "" {
			continue
		}
		mode := ""
		if len(mount.Options) > 0 {
			mode = ":" + strings.Join(mount.Options, "")
		}
		spec.HostConfig.Binds = append(spec.HostConfig.Binds, fmt.Sprintf("%s:%s%s", mount.Source, mount.Destination, mode))
	}
	// Volumes
	for _, mount := range newContainer.Mounts {
		if mount.Name == "" || mount.Destination == "" {
			continue
		}
		mode := ""
		if mount.Mode != "" {
			mode = ":" + mount.Mode
		}
		spec.HostConfig.Binds = append(spec.HostConfig.Binds, fmt.Sprintf("%s:%s%s", mount.Name, mount.Destination, mode))
	}
	if newContainer.Annotations != nil && newContainer.Annotations["io.podman.annotations.label"] == "disable" {
		spec.HostConfig.SecurityOpt = []string{"label=disable"}
	}
	return spec
}

func mapToSlice(m map[string]string) []string {
	s := make([]string, 0)
	for k, v := range m {
		s = append(s, fmt.Sprintf("%s=%s", k, v))
	}
	return s
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
func (c *CompatClient) ContainerUpdate(name string, fn func(newContainer *container.Container)) (*container.Container, error) {
	if fn == nil {
		return nil, fmt.Errorf("at least one customization is needed")
	}

	datetime := time.Now().Format("20060102150405")
	updContainer, err := c.ContainerInspect(name)
	if err != nil {
		return nil, err
	}

	newContainerName := fmt.Sprintf("%s-new-%s", updContainer.Name, datetime)
	cc := &container.Container{
		Name:           newContainerName,
		Image:          updContainer.Image,
		Env:            updContainer.Env,
		Labels:         updContainer.Labels,
		Annotations:    updContainer.Annotations,
		Networks:       updContainer.Networks,
		Mounts:         updContainer.Mounts,
		FileMounts:     updContainer.FileMounts,
		Ports:          updContainer.Ports,
		EntryPoint:     updContainer.EntryPoint,
		Command:        updContainer.Command,
		RestartPolicy:  updContainer.RestartPolicy,
		MaxCpus:        updContainer.MaxCpus,
		MaxMemoryBytes: updContainer.MaxMemoryBytes,
	}

	// apply new container customization
	fn(cc)
	if cc.Name != newContainerName {
		return nil, fmt.Errorf("container name cannot be changed")
	}

	// creating a new container
	err = c.ContainerCreate(cc)
	if err != nil {
		return cc, err
	}

	// rollback this one in case of failures below
	defer func() {
		if err != nil {
			_ = c.ContainerStop(cc.Name)
			_ = c.ContainerRemove(cc.Name)
		}
	}()

	// stopping current container
	err = c.ContainerStop(name)
	if err != nil {
		return cc, err
	}

	// restarting current container
	defer func() {
		if err != nil {
			_ = c.ContainerStart(name)
		}
	}()

	// renaming current container to a backup name
	backupName := fmt.Sprintf("%s-%s", name, datetime)
	err = c.ContainerRename(name, backupName)
	if err != nil {
		return cc, err
	}

	// restoring original container
	defer func() {
		if err != nil {
			_ = c.ContainerRename(backupName, name)
		}
	}()

	// renaming new container to current name
	err = c.ContainerRename(cc.Name, name)
	if err != nil {
		return cc, err
	}

	defer func() {
		if err != nil {
			_ = c.ContainerRename(name, newContainerName)
		}
	}()

	// starting new container
	err = c.ContainerStart(name)
	if err != nil {
		return cc, err
	} else {
		if removeErr := c.ContainerRemove(backupName); removeErr != nil {
			fmt.Printf("Unable to remove backup container: %s - %s", backupName, err)
			fmt.Println()
		}
	}

	return cc, nil
}

// ContainerUpdateImage updates the image used by the given container (name).
func (c *CompatClient) ContainerUpdateImage(ctx context.Context, name string, newImage string) (*container.Container, error) {
	err := c.ImagePull(ctx, newImage)
	if err != nil {
		return nil, fmt.Errorf("error pulling image %q: %s", newImage, err)
	}
	return c.ContainerUpdate(name, func(newContainer *container.Container) {
		newContainer.Image = newImage
	})
}

func (c *CompatClient) ContainerRename(currentName, newName string) error {
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerRenameParams()
	params.PathName = currentName
	params.QueryName = newName
	_, err := cli.ContainerRename(params)
	if err != nil {
		return fmt.Errorf("error renaming container %s to %s: %v", currentName, newName, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) ContainerRemove(name string) error {
	existing, err := c.ContainerInspect(name)
	if err != nil {
		return fmt.Errorf("container %s not found: %w", name, err)
	}
	if !container.IsOwnedBySkupper(existing.Labels) {
		return fmt.Errorf("container %s is not owned by Skupper", name)
	}
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerDeleteParams()
	params.Name = name
	params.Force = boolTrue()
	_, err = cli.ContainerDelete(params)
	if err != nil {
		return fmt.Errorf("error deleting container %s: %v", name, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) ContainerExec(id string, command []string) (string, error) {
	params := exec_compat.NewContainerExecParams()
	params.Name = id
	params.Control = exec_compat.ContainerExecBody{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          command,
	}
	// Creating the exec
	execOp := &runtime.ClientOperation{
		ID:                 "ContainerExec",
		Method:             "POST",
		PathPattern:        "/containers/{name}/exec",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             &responseReaderID{},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	result, err := c.RestClient.Submit(execOp)
	if err != nil {
		return "", fmt.Errorf("error executing command on %s: %v", id, ToAPIError(err))
	}
	resp, ok := result.(*models.IDResponse)
	if !ok {
		return "", fmt.Errorf("error parsing execution id")
	}

	// Starting the exec
	startParams := exec_compat.NewExecStartParams()
	startParams.ID = resp.ID

	reader := &multiplexedBodyReader{}
	startOp := &runtime.ClientOperation{
		ID:                 "ExecStart",
		Method:             "POST",
		PathPattern:        "/exec/{id}/start",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             startParams,
		Reader:             reader,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}

	restClient, ok := c.RestClient.(*runtimeclient.Runtime)
	if ok {
		restClient.Consumers["*/*"] = reader
	}
	result, err = c.RestClient.Submit(startOp)
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

func (c *CompatClient) ContainerLogs(id string) (string, error) {
	params := containers_compat.NewContainerLogsParams()
	params.Name = id
	params.Stdout = boolTrue()
	params.Stderr = boolTrue()
	reader := &responseReaderOctetStreamBody{}
	op := &runtime.ClientOperation{
		ID:                 "ContainerLogs",
		Method:             "GET",
		PathPattern:        "/containers/{name}/logs",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             reader,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	restClient, ok := c.RestClient.(*runtimeclient.Runtime)
	if ok {
		restClient.Consumers["*/*"] = reader
	}
	result, err := c.RestClient.Submit(op)
	if err != nil {
		return "", fmt.Errorf("error retrieving logs from container %s: %v", id, err)
	}
	logs := result.(string)
	return logs, nil
}

func (c *CompatClient) ContainerStart(name string) error {
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerStartParams()
	params.Name = name
	_, err := cli.ContainerStart(params)
	if err != nil {
		return fmt.Errorf("error starting container %s: %v", name, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) ContainerStop(name string) error {
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerStopParams()
	params.Name = name
	_, err := cli.ContainerStop(params)
	if err != nil {
		return fmt.Errorf("error stopping container %s: %v", name, ToAPIError(err))
	}
	return nil
}

func (c *CompatClient) ContainerRestart(name string) error {
	cli := containers_compat.New(c.RestClient, formats)
	params := containers_compat.NewContainerRestartParams()
	params.Name = name
	_, err := cli.ContainerRestart(params)
	if err != nil {
		return fmt.Errorf("error restarting container %s: %v", name, ToAPIError(err))
	}
	return nil
}
