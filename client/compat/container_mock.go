package compat

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/containers_compat"
	"github.com/skupperproject/skupper/client/generated/libpod/client/networks_compat"
	"github.com/skupperproject/skupper/client/generated/libpod/client/system_compat"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes_compat"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
	"github.com/skupperproject/skupper/pkg/config"
)

func NewCompatClientMock(containers []*container.Container) *CompatClient {
	return &CompatClient{
		RestClient: &RestClientMock{
			Containers: containers,
		},
		endpoint: "",
	}
}

// RestClientMock maintains (in memory) a set of containers
// and volumes that are manipulated through compat client.
// It also maintains a set of volume files (in disk) to mock
// podman volumes.
// The optional ErrorHook can be set by tests to modify the
// behavior of certain operations before they are actually
// performed by the mock implementation.
type RestClientMock struct {
	Containers   []*container.Container
	networks     map[string]*container.Network
	Volumes      map[string]*container.Volume
	VolumesFiles map[string]map[string]string
	volumesDir   string
	ErrorHook    func(operation *runtime.ClientOperation) error
}

func (r *RestClientMock) MockVolumeFiles(volumes map[string]*container.Volume, volumesFiles map[string]map[string]string) error {
	var err error
	r.volumesDir, err = os.MkdirTemp("", "skupper-mock-")
	if err != nil {
		return err
	}
	r.Volumes = volumes
	r.VolumesFiles = volumesFiles

	prepareVolumeDir := func(volumeName string, volume *container.Volume) error {
		volumeDir := path.Join(r.volumesDir, volumeName)
		if err = os.Mkdir(volumeDir, 0755); err != nil {
			return err
		}
		volume.Source = volumeDir
		return nil
	}
	// creating temp directory for volume
	for volumeName, volume := range r.Volumes {
		if err = prepareVolumeDir(volumeName, volume); err != nil {
			return err
		}
	}
	// creating volume files
	for volumeName, volumeFiles := range r.VolumesFiles {
		volume, ok := r.Volumes[volumeName]
		// if volume not defined earlier, create it
		if !ok {
			v := &container.Volume{
				Name: volumeName,
			}
			r.Volumes[volumeName] = v
			if err = prepareVolumeDir(volumeName, v); err != nil {
				return err
			}

		}
		if _, err = volume.CreateFiles(volumeFiles, true); err != nil {
			return err
		}
	}
	return nil
}

func (r *RestClientMock) CleanupMockVolumeDir() error {
	if r.volumesDir != "" {
		return os.RemoveAll(r.volumesDir)
	}
	return nil
}

func (r *RestClientMock) Submit(operation *runtime.ClientOperation) (interface{}, error) {
	var res interface{}
	var err error = nil

	switch operation.ID {
	case "SystemInfo":
		res, err = r.HandleSystemInfo(operation, r.ErrorHook)
	case "ImageCreate":
		res, err = r.HandleImageCreate(operation, r.ErrorHook)
	case "ContainerList":
		res, err = r.HandleContainerList(operation, r.ErrorHook)
	case "ContainerInspect":
		res, err = r.HandleContainerInspect(operation, r.ErrorHook)
	case "ContainerCreate":
		res, err = r.HandleContainerCreate(operation, r.ErrorHook)
	case "ContainerDelete":
		res, err = r.HandleContainerDelete(operation, r.ErrorHook)
	case "ContainerRename":
		res, err = r.HandleContainerRename(operation, r.ErrorHook)
	case "ContainerStart":
		res, err = r.HandleContainerStart(operation, r.ErrorHook)
	case "ContainerStop":
		res, err = r.HandleContainerStop(operation, r.ErrorHook)
	case "VolumeCreate":
		res, err = r.HandleVolumeCreate(operation, r.ErrorHook)
	case "VolumeInspect":
		res, err = r.HandleVolumeInspect(operation, r.ErrorHook)
	case "VolumeList":
		res, err = r.HandleVolumeList(operation, r.ErrorHook)
	case "VolumeDelete":
		res, err = r.HandleVolumeDelete(operation, r.ErrorHook)
	case "NetworkInspect":
		res, err = r.HandleNetworkInspect(operation, r.ErrorHook)
	case "NetworkCreate":
		res, err = r.HandleNetworkCreate(operation, r.ErrorHook)
	case "NetworkDelete":
		res, err = r.HandleNetworkDelete(operation, r.ErrorHook)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation.ID)
	}
	return res, err
}

func (r *RestClientMock) HandleContainerList(operation *runtime.ClientOperation, errorHook func(operation *runtime.ClientOperation) error) (*containers_compat.ContainerListOK, error) {
	res := &containers_compat.ContainerListOK{}
	if len(r.Containers) > 0 {
		res.Payload = []interface{}{}
		var list []interface{}
		for _, c := range r.Containers {
			state := "running"
			if !c.Running {
				state = "exited"
			}
			listContainer := map[string]interface{}{
				"Id": c.ID,
				"Names": []interface{}{
					c.Name,
				},
				"Image":   c.Image,
				"Labels":  asStringInterfaceMap(c.Labels),
				"Command": strings.Join(c.Command, " "),
				"Created": json.Number(strconv.FormatInt(c.CreatedAt.Unix(), 10)),
				"State":   state,
			}
			if len(c.Networks) > 0 {
				listContainer["NetworkSettings"] = map[string]interface{}{
					"Networks": func() map[string]interface{} {
						containerNetworks := make(map[string]interface{})
						for networkName, containerNetwork := range c.Networks {
							containerNetworks[networkName] = map[string]interface{}{
								"NetworkID":   containerNetwork.ID,
								"IPAddress":   containerNetwork.IPAddress,
								"IPPrefixLen": json.Number(strconv.Itoa(containerNetwork.IPPrefixLen)),
								"MacAddress":  containerNetwork.MacAddress,
								"Gateway":     containerNetwork.Gateway,
								"Aliases":     containerNetwork.Aliases,
							}
						}
						return containerNetworks
					}(),
				}
			}
			if len(c.Mounts)+len(c.FileMounts) > 0 {
				var mounts []interface{}
				for _, mount := range c.Mounts {
					newMount := map[string]interface{}{
						"Type":        "volume",
						"Name":        mount.Name,
						"Source":      mount.Source,
						"Destination": mount.Destination,
						"Mode":        mount.Mode,
						"RW":          true,
					}
					mounts = append(mounts, newMount)
				}
				for _, mount := range c.FileMounts {
					newMount := map[string]interface{}{
						"Type":        "bind",
						"Source":      mount.Source,
						"Destination": mount.Destination,
					}
					mounts = append(mounts, newMount)
				}
				listContainer["Mounts"] = mounts
			}
			if len(c.Ports) > 0 {
				var ports []interface{}
				for _, port := range c.Ports {
					newPort := map[string]interface{}{
						"Type":        port.Protocol,
						"IP":          port.HostIP,
						"PublicPort":  port.Host,
						"PrivatePort": port.Target,
					}
					ports = append(ports, newPort)
				}
				listContainer["Ports"] = ports
			}
			list = append(list, listContainer)
		}
		res.Payload = list
	}
	if errorHook != nil {
		return res, errorHook(operation)
	}
	return res, nil
}

func (r *RestClientMock) HandleContainerInspect(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (*containers_compat.ContainerInspectOK, error) {
	res := &containers_compat.ContainerInspectOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers_compat.ContainerInspectParams)
	var c *container.Container
	for _, cc := range r.Containers {
		if cc.Name == params.Name || cc.ID == params.Name {
			c = cc
			break
		}
	}
	if c == nil {
		return nil, fmt.Errorf("container not found")
	}
	userId := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	platform := config.GetPlatform()
	if platform == types.PlatformDocker {
		userId = "0:0"
	}

	res.Payload = &containers_compat.ContainerInspectOKBody{
		Config: &models.Config{
			Cmd:        c.Command,
			Entrypoint: c.EntryPoint,
			Env:        c.EnvSlice(),
			Image:      c.Image,
			Labels:     c.Labels,
			User:       userId,
		},
		Created: strfmt.DateTime(c.CreatedAt).String(),
		ID:      c.ID,
		Image:   c.Image,
		Mounts: func() []*models.MountPoint {
			var m []*models.MountPoint
			for _, mount := range c.Mounts {
				m = append(m, &models.MountPoint{
					Name:        mount.Name,
					Source:      mount.Source,
					Destination: mount.Destination,
					Type:        "volume",
					Mode:        mount.Mode,
					RW:          mount.RW,
				})
			}
			for _, fm := range c.FileMounts {
				m = append(m, &models.MountPoint{
					Source:      fm.Source,
					Destination: fm.Destination,
					Propagation: models.Propagation(fm.Propagation),
					Type:        "bind",
				})
			}
			return m
		}(),
		Name: c.Name,
		NetworkSettings: func() *models.NetworkSettings {
			if len(c.Networks) == 0 && len(c.Ports) == 0 {
				return nil
			}
			settings := &models.NetworkSettings{
				Networks: map[string]models.EndpointSettings{},
				Ports:    models.PortMap{},
			}
			for k, v := range c.Networks {
				settings.Networks[k] = models.EndpointSettings{
					NetworkID:   "",
					IPAddress:   "",
					IPPrefixLen: 0,
					MacAddress:  "",
					Gateway:     "",
					Aliases:     v.Aliases,
				}
			}
			for _, port := range c.Ports {
				portKey := fmt.Sprintf("%s/%s", port.Target, port.Protocol)
				settings.Ports[portKey] = []models.PortBinding{
					{HostIP: port.HostIP, HostPort: port.Host},
				}
			}
			return settings
		}(),
		State: &models.ContainerState{
			ExitCode:   int64(c.ExitCode),
			FinishedAt: strfmt.DateTime(c.ExitedAt).String(),
			Running:    c.Running,
			StartedAt:  strfmt.DateTime(c.StartedAt).String(),
		},
	}
	if c.MaxCpus > 0 || c.MaxMemoryBytes > 0 {
		res.Payload.HostConfig = &models.HostConfig{
			CPUQuota:  int64(c.MaxCpus * 100000),
			CPUPeriod: 100000,
			Memory:    c.MaxMemoryBytes,
		}
	}
	return res, nil
}

func (r *RestClientMock) HandleContainerCreate(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (*containers_compat.ContainerCreateCreated, error) {
	res := &containers_compat.ContainerCreateCreated{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers_compat.ContainerCreateParams)
	spec := params.Body

	for _, c := range r.Containers {
		if c.Name == spec.Name {
			return res, fmt.Errorf("container already exists")
		}
	}
	c := &container.Container{
		ID:     strings.Replace(uuid.New().String(), "-", "", -1),
		Name:   spec.Name,
		Image:  spec.Image,
		Labels: spec.Labels,
		Annotations: func() map[string]string {
			annotations := map[string]string{}
			for _, opt := range spec.HostConfig.SecurityOpt {
				if opt == "label=disable" {
					annotations["io.podman.annotations.label"] = "disable"
				}
			}
			return annotations
		}(),
		Networks: func() map[string]container.ContainerNetworkInfo {
			res := map[string]container.ContainerNetworkInfo{}
			if spec.NetworkingConfig != nil {
				for network, networkConfig := range spec.NetworkingConfig.EndpointsConfig {
					res[network] = container.ContainerNetworkInfo{
						ID:          networkConfig.NetworkID,
						IPAddress:   networkConfig.IPAddress,
						IPPrefixLen: int(networkConfig.IPPrefixLen),
						MacAddress:  networkConfig.MacAddress,
						Gateway:     networkConfig.Gateway,
						Aliases:     networkConfig.Aliases,
					}
				}
			}
			return res
		}(),
		Mounts: func() []container.Volume {
			var res []container.Volume
			for _, v := range spec.HostConfig.Binds {
				bindSplit := strings.Split(v, ":")
				isFileMount := strings.Contains(bindSplit[0], "/")
				if isFileMount {
					continue
				}
				var mode string
				if len(bindSplit) == 3 {
					mode = bindSplit[2]
				}
				res = append(res, container.Volume{
					Name:        bindSplit[0],
					Destination: bindSplit[1],
					Mode:        mode,
				})
			}
			return res
		}(),
		FileMounts: func() []container.FileMount {
			var res []container.FileMount
			for _, v := range spec.HostConfig.Binds {
				bindSplit := strings.Split(v, ":")
				isVolumeName := !strings.Contains(bindSplit[0], "/")
				if isVolumeName {
					continue
				}
				var mode string
				if len(bindSplit) == 3 {
					mode = bindSplit[2]
				}
				res = append(res, container.FileMount{
					Source:      bindSplit[0],
					Destination: bindSplit[1],
					Options:     []string{mode},
				})
			}
			return res
		}(),
		Ports: func() []container.Port {
			var res []container.Port
			// Port mapping
			for target, bindings := range spec.HostConfig.PortBindings {
				targetProto := strings.Split(target, "/")
				for _, binding := range bindings {
					res = append(res, container.Port{
						Host:     binding.HostPort,
						HostIP:   binding.HostIP,
						Target:   targetProto[0],
						Protocol: targetProto[1],
					})
				}
			}
			return res
		}(),
		EntryPoint: spec.Entrypoint,
		Command:    spec.Cmd,
		RestartPolicy: func() string {
			if spec.HostConfig.RestartPolicy != nil {
				return spec.HostConfig.RestartPolicy.Name
			}
			return ""
		}(),
		CreatedAt: time.Now(),
	}
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}
	c.Labels["application"] = types.AppName
	c.FromEnv(spec.Env)
	c.MaxCpus = int(spec.HostConfig.CPUCount)
	c.MaxMemoryBytes = spec.HostConfig.Memory
	r.Containers = append(r.Containers, c)
	return res, nil
}

func (r *RestClientMock) HandleContainerRename(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (*containers_compat.ContainerRenameNoContent, error) {
	res := &containers_compat.ContainerRenameNoContent{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers_compat.ContainerRenameParams)
	curName := params.PathName
	newName := params.QueryName

	// current name must exist
	var cc *container.Container

	for _, c := range r.Containers {
		if c.Name == newName {
			return res, fmt.Errorf("a container named %q already exists", newName)
		} else if c.Name == curName {
			cc = c
		}
	}
	cc.Name = newName
	return res, nil
}

func (r *RestClientMock) HandleContainerStart(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	res := &containers_compat.ContainerStartNoContent{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers_compat.ContainerStartParams)
	for _, c := range r.Containers {
		if c.Name == params.Name || c.ID == params.Name {
			if !c.Running {
				c.Running = true
				c.StartedAt = time.Now()
				c.ExitCode = 0
				c.ExitedAt = time.Time{}
			}
			return res, nil
		}
	}
	return res, fmt.Errorf("no container with name or ID %q", params.Name)
}

func (r *RestClientMock) HandleContainerStop(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res interface{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers_compat.ContainerStopParams)
	for _, c := range r.Containers {
		if c.Name == params.Name || c.ID == params.Name {
			res = &containers_compat.ContainerStopNoContent{}
			if c.Running {
				c.Running = false
				c.ExitedAt = time.Now()
				c.ExitCode = 0
			}
			return res, nil
		}
	}
	res = &containers_compat.ContainerStopNotFound{}
	return res, fmt.Errorf("no container with name or ID %q", params.Name)
}

func (r *RestClientMock) HandleContainerDelete(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res interface{}
	if hook != nil {
		if err := hook(operation); err != nil {
			res = &containers_compat.ContainerDeleteNoContent{}
			return res, err
		}
	}
	params := operation.Params.(*containers_compat.ContainerDeleteParams)
	for i, c := range r.Containers {
		if c.Name == params.Name || c.ID == params.Name {
			r.Containers = append(r.Containers[:i], r.Containers[i+1:]...)
			res = &containers_compat.ContainerDeleteNoContent{}
			return res, nil
		}
	}
	res = &containers_compat.ContainerDeleteNoContent{}
	return res, fmt.Errorf("no container with name or ID %q", params.Name)
}

func (r *RestClientMock) HandleVolumeInspect(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &volumes_compat.VolumeInspectOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*volumes_compat.VolumeInspectParams)
	v, ok := r.Volumes[params.Name]
	if !ok {
		return res, &volumes_compat.VolumeInspectNotFound{
			Payload: &volumes_compat.VolumeInspectNotFoundBody{
				Message:      fmt.Sprintf("no volume with name %q", params.Name),
				ResponseCode: 404,
			},
		}
	}
	res.Payload = &volumes_compat.VolumeInspectOKBody{
		Labels:     v.Labels,
		Mountpoint: &v.Source,
		Name:       &v.Name,
	}
	return res, nil
}

func (r *RestClientMock) HandleVolumeList(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &models.VolumeListOKBody{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	for vName, v := range r.Volumes {
		res.Volumes = append(res.Volumes, &models.Volume{
			Labels:     v.Labels,
			Mountpoint: &v.Source,
			Name:       &vName,
		})
	}
	return res, nil
}

func (r *RestClientMock) HandleVolumeDelete(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &volumes_compat.VolumeDeleteNoContent{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*volumes_compat.VolumeDeleteParams)
	v, ok := r.Volumes[params.Name]
	if !ok {
		return res, fmt.Errorf("no volume with name %q", params.Name)
	}
	delete(r.Volumes, v.Name)
	return res, nil
}

func (r *RestClientMock) HandleNetworkInspect(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &networks_compat.NetworkInspectOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*networks_compat.NetworkInspectParams)
	n, ok := r.networks[params.Name]
	if !ok {
		return res, fmt.Errorf("no network with name %q", params.Name)
	}

	subnets := []*models.Subnet{}
	for _, sn := range n.Subnets {
		subnets = append(subnets, &models.Subnet{
			Gateway: sn.Gateway,
			Subnet:  sn.Subnet,
		})
	}

	created, _ := strfmt.ParseDateTime(n.CreatedAt)
	res.Payload = &models.NetworkResource{
		Created:    created,
		Driver:     n.Driver,
		ID:         n.ID,
		EnableIPV6: n.IPV6,
		Internal:   n.Internal,
		Labels:     n.Labels,
		Name:       n.Name,
		Options:    n.Options,
	}
	res.Payload.IPAM = &models.IPAM{}
	for _, ss := range n.Subnets {
		res.Payload.IPAM.Config = append(res.Payload.IPAM.Config, &models.IPAMConfig{
			Gateway: ss.Gateway,
			Subnet:  ss.Subnet,
		})
	}

	return res, nil
}

func (r *RestClientMock) HandleSystemInfo(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &system_compat.SystemVersionOK{
		Payload: &system_compat.SystemVersionOKBody{
			Version:       "4.0.0",
			APIVersion:    "4.0.0",
			Arch:          "amd64",
			KernelVersion: "6.6",
			Os:            "linux",
			Components: []*models.ComponentVersion{
				{
					Details: map[string]string{
						"APIVersion": "4.0.0",
					},
					Name: "podman",
				},
			},
		},
	}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (r *RestClientMock) HandleNetworkCreate(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &networkCreateOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	if r.networks == nil {
		r.networks = map[string]*container.Network{}
	}

	params := operation.Params.(*networks_compat.NetworkCreateParams)
	_, ok := r.networks[params.Create.Name]
	if ok {
		return res, fmt.Errorf("network already exists %q", params.Create.Name)
	}
	n := params.Create

	nn := &container.Network{
		ID:        uuid.NewString(),
		Name:      n.Name,
		Driver:    n.Driver,
		IPV6:      n.EnableIPV6,
		DNS:       true,
		Internal:  n.Internal,
		Labels:    n.Labels,
		Options:   n.Options,
		CreatedAt: strfmt.DateTime(time.Now()).String(),
	}
	if n.IPAM != nil {
		for _, s := range n.IPAM.Config {
			nn.Subnets = append(nn.Subnets, &container.Subnet{
				Subnet:  s.Subnet,
				Gateway: s.Gateway,
			})
		}
	}
	r.networks[nn.Name] = nn

	res.ID = &nn.ID
	return res, nil
}

func (r *RestClientMock) HandleVolumeCreate(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &volumes_compat.VolumeCreateCreated{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*volumes_compat.VolumeCreateParams)
	v := params.Create
	_, ok := r.Volumes[*v.Name]
	if ok {
		return res, fmt.Errorf("volume %q already exists", *v.Name)
	}

	vDir, err := os.MkdirTemp("", "skupper-mock-")
	if err != nil {
		return res, err
	}
	if r.Volumes == nil {
		r.Volumes = map[string]*container.Volume{}
	}
	r.Volumes[*v.Name] = &container.Volume{
		Name:   *v.Name,
		Source: vDir,
		Labels: v.Labels,
	}
	res.Payload = &volumes_compat.VolumeCreateCreatedBody{
		CreatedAt:  strfmt.DateTime(time.Now()).String(),
		Mountpoint: &vDir,
		Driver:     v.Driver,
		Labels:     v.Labels,
		Name:       v.Name,
	}
	return res, nil
}

func (r *RestClientMock) HandleNetworkDelete(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &networks_compat.NetworkDeleteNoContent{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	if r.networks == nil {
		r.networks = map[string]*container.Network{}
	}

	params := operation.Params.(*networks_compat.NetworkDeleteParams)
	_, ok := r.networks[params.Name]
	if !ok {
		return res, fmt.Errorf("network %q not found", params.Name)
	}
	delete(r.networks, params.Name)
	return res, nil
}

func (r *RestClientMock) HandleImageCreate(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	if hook != nil {
		if err := hook(operation); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func mockContainers(image string) []*container.Container {
	return []*container.Container{{
		ID:    "abcd",
		Name:  "my-container",
		Image: image,
		Env:   map[string]string{"var1": "val1", "var2": "val2"},
		Labels: map[string]string{
			"application": types.AppName,
		},
		Networks: map[string]container.ContainerNetworkInfo{
			"skupper": container.ContainerNetworkInfo{
				ID:        "skupper",
				IPAddress: "172.17.0.10",
				Gateway:   "172.17.0.1",
				Aliases:   []string{"skupper", "service-controller"},
			},
		},
		Ports: []container.Port{
			{Host: "8888", HostIP: "10.0.0.1", Target: "8080", Protocol: "tcp"},
		},
		Running:   true,
		CreatedAt: time.Now(),
		StartedAt: time.Now(),
	}}
}
