package podman

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/skupperproject/skupper/client/generated/libpod/client/containers"
	"github.com/skupperproject/skupper/client/generated/libpod/client/networks"
	"github.com/skupperproject/skupper/client/generated/libpod/client/system"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes"
	"github.com/skupperproject/skupper/client/generated/libpod/models"
	"github.com/skupperproject/skupper/pkg/container"
)

func NewPodmanClientMock(containers []*container.Container) *PodmanRestClient {
	return &PodmanRestClient{
		RestClient: &RestClientMock{
			Containers: containers,
		},
		endpoint: "",
	}
}

// RestClientMock maintains (in memory) a set of containers
// and volumes that are manipulated through libpod client.
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
	case "SystemInfoLibpod":
		res, err = r.HandleSystemInfo(operation, r.ErrorHook)
	case "ContainerListLibpod":
		res, err = r.HandleContainerList(operation, r.ErrorHook)
	case "ContainerInspectLibpod":
		res, err = r.HandleContainerInspect(operation, r.ErrorHook)
	case "ContainerCreateLibpod":
		res, err = r.HandleContainerCreate(operation, r.ErrorHook)
	case "ContainerDeleteLibpod":
		res, err = r.HandleContainerDelete(operation, r.ErrorHook)
	case "ContainerRenameLibpod":
		res, err = r.HandleContainerRename(operation, r.ErrorHook)
	case "ContainerStartLibpod":
		res, err = r.HandleContainerStart(operation, r.ErrorHook)
	case "ContainerStopLibpod":
		res, err = r.HandleContainerStop(operation, r.ErrorHook)
	case "VolumeCreateLibpod":
		res, err = r.HandleVolumeCreate(operation, r.ErrorHook)
	case "VolumeInspectLibpod":
		res, err = r.HandleVolumeInspect(operation, r.ErrorHook)
	case "VolumeListLibpod":
		res, err = r.HandleVolumeList(operation, r.ErrorHook)
	case "VolumeDeleteLibpod":
		res, err = r.HandleVolumeDelete(operation, r.ErrorHook)
	case "NetworkInspectLibpod":
		res, err = r.HandleNetworkInspect(operation, r.ErrorHook)
	case "NetworkCreateLibpod":
		res, err = r.HandleNetworkCreate(operation, r.ErrorHook)
	case "NetworkDeleteLibpod":
		res, err = r.HandleNetworkDelete(operation, r.ErrorHook)
	}
	return res, err
}

func (r *RestClientMock) HandleContainerList(operation *runtime.ClientOperation, errorHook func(operation *runtime.ClientOperation) error) (*containers.ContainerListLibpodOK, error) {
	res := &containers.ContainerListLibpodOK{}
	if len(r.Containers) > 0 {
		res.Payload = []*models.ListContainer{}
		for _, c := range r.Containers {
			state := "running"
			if !c.Running {
				state = "exited"
			}
			res.Payload = append(res.Payload, &models.ListContainer{
				Command:   c.Command,
				Created:   strfmt.DateTime(c.CreatedAt),
				CreatedAt: c.CreatedAt.String(),
				ExitCode:  int32(c.ExitCode),
				Exited:    !c.ExitedAt.Equal(time.Time{}),
				ExitedAt:  c.ExitedAt.Unix(),
				ID:        c.ID,
				Image:     c.Image,
				ImageID:   c.Image,
				Labels:    c.Labels,
				Mounts: func() []string {
					var mounts []string
					for _, m := range c.Mounts {
						mounts = append(mounts, m.Destination)
					}
					return mounts
				}(),
				Names: []string{c.Name},
				Networks: func() []string {
					var networks []string
					for _, n := range c.Networks {
						networks = append(networks, n.ID)
					}
					return networks
				}(),
				Pod: c.Pod,
				Ports: func() []*models.PortMapping {
					var pm []*models.PortMapping
					for _, port := range c.Ports {
						containerPort, _ := strconv.ParseUint(port.Target, 10, 16)
						hostPort, _ := strconv.ParseUint(port.Host, 10, 16)
						pm = append(pm, &models.PortMapping{
							ContainerPort: uint16(containerPort),
							HostIP:        port.HostIP,
							HostPort:      uint16(hostPort),
							Protocol:      port.Protocol,
						})
					}
					return pm
				}(),
				StartedAt: c.StartedAt.Unix(),
				State:     state,
			})
		}
	}
	if errorHook != nil {
		return res, errorHook(operation)
	}
	return res, nil
}

func (r *RestClientMock) HandleContainerInspect(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (*containers.ContainerInspectLibpodOK, error) {
	res := &containers.ContainerInspectLibpodOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers.ContainerInspectLibpodParams)
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
	res.Payload = &containers.ContainerInspectLibpodOKBody{
		Config: &models.InspectContainerConfig{
			Annotations: c.Annotations,
			Cmd:         c.Command,
			Entrypoint:  strings.Join(c.EntryPoint, " "),
			Env:         c.EnvSlice(),
			Image:       c.Image,
			Labels:      c.Labels,
			User:        ToSpecGenerator(c).User,
		},
		Created:   strfmt.DateTime(c.CreatedAt),
		ID:        c.ID,
		Image:     c.Image,
		ImageName: c.Image,
		Mounts: func() []*models.InspectMount {
			var m []*models.InspectMount
			for _, mount := range c.Mounts {
				m = append(m, &models.InspectMount{
					Name:        mount.Name,
					Source:      mount.Source,
					Destination: mount.Destination,
					Type:        "volume",
					Mode:        mount.Mode,
					RW:          mount.RW,
				})
			}
			for _, fm := range c.FileMounts {
				m = append(m, &models.InspectMount{
					Source:      fm.Source,
					Destination: fm.Destination,
					Options:     fm.Options,
					Type:        "bind",
				})
			}
			return m
		}(),
		Name: c.Name,
		NetworkSettings: func() *models.InspectNetworkSettings {
			if len(c.Networks) == 0 && len(c.Ports) == 0 {
				return nil
			}
			settings := &models.InspectNetworkSettings{
				Networks: map[string]models.InspectAdditionalNetwork{},
				Ports:    map[string][]models.InspectHostPort{},
			}
			for k, v := range c.Networks {
				settings.Networks[k] = models.InspectAdditionalNetwork{
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
				settings.Ports[portKey] = []models.InspectHostPort{
					{HostIP: port.HostIP, HostPort: port.Host},
				}
			}
			return settings
		}(),
		Pod: c.Pod,
		State: &models.InspectContainerState{
			ExitCode:   int32(c.ExitCode),
			FinishedAt: strfmt.DateTime(c.ExitedAt),
			Running:    c.Running,
			StartedAt:  strfmt.DateTime(c.StartedAt),
		},
	}
	if c.MaxCpus > 0 || c.MaxMemoryBytes > 0 {
		res.Payload.HostConfig = &models.InspectContainerHostConfig{
			CPUQuota:  int64(c.MaxCpus * 100000),
			CPUPeriod: 100000,
			Memory:    c.MaxMemoryBytes,
		}
	}
	return res, nil
}

func (r *RestClientMock) HandleContainerCreate(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (*containers.ContainerCreateLibpodCreated, error) {
	res := &containers.ContainerCreateLibpodCreated{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers.ContainerCreateLibpodParams)
	spec := params.Create

	for _, c := range r.Containers {
		if c.Name == spec.Name {
			return res, fmt.Errorf("container already exists")
		}
	}
	c := &container.Container{
		ID:          strings.Replace(uuid.New().String(), "-", "", -1),
		Name:        spec.Name,
		Pod:         spec.Pod,
		Image:       spec.Image,
		Env:         spec.Env,
		Labels:      spec.Labels,
		Annotations: spec.Annotations,
		Networks: func() map[string]container.ContainerNetworkInfo {
			res := map[string]container.ContainerNetworkInfo{}
			for name, perNetworkOpts := range spec.Networks {
				res[name] = container.ContainerNetworkInfo{
					Aliases: perNetworkOpts.Aliases,
				}
			}
			return res
		}(),
		Mounts: func() []container.Volume {
			var res []container.Volume
			for _, v := range spec.Volumes {
				res = append(res, container.Volume{
					Name:        v.Name,
					Destination: v.Dest,
				})
			}
			return res
		}(),
		FileMounts: func() []container.FileMount {
			var res []container.FileMount
			for _, fm := range spec.Mounts {
				res = append(res, container.FileMount{
					Source:      fm.Source,
					Destination: fm.Destination,
					Options:     fm.Options,
				})
			}
			return nil
		}(),
		Ports: func() []container.Port {
			var res []container.Port
			// Port mapping
			for _, pm := range spec.PortMappings {
				res = append(res, container.Port{
					Host:     strconv.Itoa(int(pm.HostPort)),
					HostIP:   pm.HostIP,
					Target:   strconv.Itoa(int(pm.ContainerPort)),
					Protocol: pm.Protocol,
				})
			}
			return res
		}(),
		EntryPoint:    spec.Entrypoint,
		Command:       spec.Command,
		RestartPolicy: spec.RestartPolicy,
		RestartCount:  int(spec.RestartRetries),
		CreatedAt:     time.Now(),
	}
	if spec.ResourceLimits != nil {
		if spec.ResourceLimits.CPU != nil {
			c.MaxCpus = int(spec.ResourceLimits.CPU.Quota / 100000)
		}
		if spec.ResourceLimits.Memory != nil {
			c.MaxMemoryBytes = spec.ResourceLimits.Memory.Limit
		}
	}
	r.Containers = append(r.Containers, c)
	return res, nil
}

func (r *RestClientMock) HandleContainerRename(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (*containers.ContainerRenameLibpodNoContent, error) {
	res := &containers.ContainerRenameLibpodNoContent{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers.ContainerRenameLibpodParams)
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
	res := &containers.ContainerStartLibpodNoContent{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*containers.ContainerStartLibpodParams)
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
	params := operation.Params.(*containers.ContainerStopLibpodParams)
	for _, c := range r.Containers {
		if c.Name == params.Name || c.ID == params.Name {
			res = &containers.ContainerStopLibpodNoContent{}
			if c.Running {
				c.Running = false
				c.ExitedAt = time.Now()
				c.ExitCode = 0
			}
			return res, nil
		}
	}
	res = &containers.ContainerStopLibpodNotFound{}
	return res, fmt.Errorf("no container with name or ID %q", params.Name)
}

func (r *RestClientMock) HandleContainerDelete(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res interface{}
	if hook != nil {
		if err := hook(operation); err != nil {
			res = &containers.ContainerDeleteLibpodNoContent{}
			return res, err
		}
	}
	params := operation.Params.(*containers.ContainerDeleteLibpodParams)
	for i, c := range r.Containers {
		if c.Name == params.Name || c.ID == params.Name {
			r.Containers = append(r.Containers[:i], r.Containers[i+1:]...)
			res = &containers.ContainerDeleteLibpodOK{}
			return res, nil
		}
	}
	res = &containers.ContainerDeleteLibpodNoContent{}
	return res, fmt.Errorf("no container with name or ID %q", params.Name)
}

func (r *RestClientMock) HandleVolumeInspect(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &volumes.VolumeInspectLibpodOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*volumes.VolumeInspectLibpodParams)
	v, ok := r.Volumes[params.Name]
	if !ok {
		return res, &volumes.VolumeInspectLibpodNotFound{
			Payload: &volumes.VolumeInspectLibpodNotFoundBody{
				Message:      fmt.Sprintf("no volume with name %q", params.Name),
				ResponseCode: 404,
			},
		}
	}
	res.Payload = &volumes.VolumeInspectLibpodOKBody{
		Labels:     v.Labels,
		Mountpoint: v.Source,
		Name:       v.Name,
	}
	return res, nil
}

func (r *RestClientMock) HandleVolumeList(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &volumes.VolumeListLibpodOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	for vName, v := range r.Volumes {
		res.Payload = append(res.Payload, &models.VolumeConfigResponse{
			Labels:     v.Labels,
			Mountpoint: v.Source,
			Name:       vName,
		})
	}
	return res, nil
}

func (r *RestClientMock) HandleVolumeDelete(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &volumes.VolumeDeleteLibpodNoContent{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*volumes.VolumeDeleteLibpodParams)
	v, ok := r.Volumes[params.Name]
	if !ok {
		return res, fmt.Errorf("no volume with name %q", params.Name)
	}
	delete(r.Volumes, v.Name)
	return res, nil
}

func (r *RestClientMock) HandleNetworkInspect(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &networks.NetworkInspectLibpodOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*networks.NetworkInspectLibpodParams)
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
	res.Payload = &models.Network{
		Created:     created,
		DNSEnabled:  n.DNS,
		Driver:      n.Driver,
		ID:          n.ID,
		IPV6Enabled: n.IPV6,
		Internal:    n.Internal,
		Labels:      n.Labels,
		Name:        n.Name,
		Options:     n.Options,
		Subnets:     subnets,
	}

	return res, nil
}

func (r *RestClientMock) HandleSystemInfo(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &system.SystemInfoLibpodOK{
		Payload: &models.Info{
			Version: &models.Version{
				APIVersion: "4.0.0",
				Version:    "4.0.0",
			},
			Host: &models.HostInfo{
				Arch:     "amd64",
				Hostname: "host.domain",
				Kernel:   "6.6",
				OS:       "linux",
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
	var res = &networks.NetworkCreateLibpodOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	if r.networks == nil {
		r.networks = map[string]*container.Network{}
	}

	params := operation.Params.(*networks.NetworkCreateLibpodParams)
	_, ok := r.networks[params.Create.Name]
	if ok {
		return res, fmt.Errorf("network already exists %q", params.Create.Name)
	}
	n := params.Create

	nn := &container.Network{
		ID:        n.ID,
		Name:      n.Name,
		Driver:    n.Driver,
		IPV6:      n.IPV6Enabled,
		DNS:       n.DNSEnabled,
		Internal:  n.Internal,
		Labels:    n.Labels,
		Options:   n.Options,
		CreatedAt: n.Created.String(),
	}
	for _, s := range n.Subnets {
		nn.Subnets = append(nn.Subnets, &container.Subnet{
			Subnet:  s.Subnet,
			Gateway: s.Gateway,
		})
	}
	r.networks[nn.Name] = nn

	res.Payload = &models.Network{
		Created:          n.Created,
		DNSEnabled:       n.DNSEnabled,
		Driver:           n.Driver,
		ID:               n.ID,
		IPAMOptions:      n.IPAMOptions,
		IPV6Enabled:      n.IPV6Enabled,
		Internal:         n.Internal,
		Labels:           n.Labels,
		Name:             n.Name,
		NetworkInterface: n.NetworkInterface,
		Options:          n.Options,
		Subnets:          n.Subnets,
	}
	return res, nil
}

func (r *RestClientMock) HandleVolumeCreate(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &volumes.VolumeCreateLibpodCreated{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	params := operation.Params.(*volumes.VolumeCreateLibpodParams)
	v := params.Create
	_, ok := r.Volumes[v.Name]
	if ok {
		return res, fmt.Errorf("volume %q already exists", v.Name)
	}

	vDir, err := os.MkdirTemp("", "skupper-mock-")
	if err != nil {
		return res, err
	}
	if r.Volumes == nil {
		r.Volumes = map[string]*container.Volume{}
	}
	r.Volumes[v.Name] = &container.Volume{
		Name:   v.Name,
		Source: vDir,
		Labels: v.Labels,
	}
	res.Payload = &volumes.VolumeCreateLibpodCreatedBody{
		CreatedAt:  strfmt.DateTime(time.Now()),
		Mountpoint: vDir,
		Driver:     v.Driver,
		Labels:     v.Labels,
		Name:       v.Name,
		Options:    v.Options,
	}
	return res, nil
}

func (r *RestClientMock) HandleNetworkDelete(operation *runtime.ClientOperation, hook func(operation *runtime.ClientOperation) error) (interface{}, error) {
	var res = &networks.NetworkDeleteLibpodOK{}
	if hook != nil {
		if err := hook(operation); err != nil {
			return res, err
		}
	}
	if r.networks == nil {
		r.networks = map[string]*container.Network{}
	}

	params := operation.Params.(*networks.NetworkDeleteLibpodParams)
	_, ok := r.networks[params.Name]
	if !ok {
		return res, fmt.Errorf("network %q not found", params.Name)
	}
	for _, c := range r.Containers {
		if _, ok := c.Networks[params.Name]; ok && !*params.Force {
			return res, fmt.Errorf("network %q in use by container %q", params.Name, c.Name)
		}
	}
	delete(r.networks, params.Name)
	return res, nil
}
