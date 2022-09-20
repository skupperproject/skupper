package container

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const (
	ContainerNetworkName = "skupper"
)

type Client interface {
	Version() (*Version, error)
	ContainerList() ([]*Container, error)
	ContainerInspect(id string) (*Container, error)
	ContainerCreate(container *Container) error
	ContainerRemove(id string) error
	ContainerExec(id string, command []string) (string, error)
	ContainerLogs(id string) (string, error)
	ContainerStart(id string) error
	ContainerStop(id string) error
	ContainerRestart(id string) error
	ImageList() ([]*Image, error)
	ImageInspect(id string) (*Image, error)
	ImagePull(id string) error
	NetworkList() ([]*Network, error)
	NetworkInspect(id string) (*Network, error)
	NetworkCreate(network *Network) (*Network, error)
	NetworkRemove(id string) error
	NetworkConnect(id, container string, aliases ...string) error
	NetworkDisconnect(id, container string) error
	VolumeCreate(volume *Volume) (*Volume, error)
	VolumeInspect(id string) (*Volume, error)
	VolumeRemove(id string) error
	VolumeList() ([]*Volume, error)
}

type VersionInfo struct {
	Version    string
	APIVersion string
}

type Version struct {
	Client VersionInfo
	Server VersionInfo
}

type Container struct {
	ID            string
	Name          string
	Pod           string
	Image         string
	Env           map[string]string
	Labels        map[string]string
	Annotations   map[string]string
	Networks      map[string]ContainerNetworkInfo
	Mounts        []Volume
	Ports         []Port
	EntryPoint    []string
	Command       []string
	RestartPolicy string
	RestartCount  int
	Running       bool
	CreatedAt     string
	StartedAt     string
	ExitedAt      string
	ExitCode      int
}

func (c *Container) FromEnv(env []string) {
	for _, e := range env {
		if !strings.Contains(e, "=") {
			continue
		}
		es := strings.SplitN(e, "=", 2)
		c.Env[es[0]] = es[1]
	}
}

func (c *Container) EnvSlice() []string {
	es := []string{}
	for k, v := range c.Env {
		es = append(es, fmt.Sprintf("%s=%s", k, v))
	}
	return es
}

func (c *Container) NetworkNames() []string {
	var networks []string
	for name, _ := range c.Networks {
		networks = append(networks, name)
	}
	return networks
}

func (c *Container) NetworkAliases() map[string][]string {
	netNames := map[string][]string{}
	for name, net := range c.Networks {
		netNames[name] = net.Aliases
	}
	return netNames
}

type Volume struct {
	Name        string
	Source      string
	Destination string
	Mode        string
	RW          bool
	Labels      map[string]string
}

func (v *Volume) GetLabels() map[string]string {
	if v.Labels == nil {
		v.Labels = map[string]string{}
	}
	return v.Labels
}

func (v *Volume) getVolumeDir() (*os.File, error) {
	if v.Source == "" {
		return nil, nil
	}
	vDir, err := os.Open(v.Source)
	if err != nil {
		return nil, err
	}
	stat, err := vDir.Stat()
	if err != nil || !stat.IsDir() {
		// list only works against the host machine
		return nil, fmt.Errorf("this is not a local volume")
	}
	return vDir, nil
}

func (v *Volume) ListFiles() ([]os.DirEntry, error) {
	vDir, err := v.getVolumeDir()
	if err != nil {
		return nil, err
	}
	files, err := vDir.ReadDir(0)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (v *Volume) ReadFile(name string) (string, error) {
	vDir, err := v.getVolumeDir()
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadFile(path.Join(vDir.Name(), name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (v *Volume) CreateFiles(fileData map[string]string, overwrite bool) ([]*os.File, error) {
	dataMap := map[string][]byte{}
	for k, v := range fileData {
		dataMap[k] = []byte(v)
	}
	return v.CreateDataFiles(dataMap, overwrite)
}

func (v *Volume) CreateDataFiles(fileData map[string][]byte, overwrite bool) ([]*os.File, error) {
	files := []*os.File{}
	for fileName, data := range fileData {
		f, err := v.CreateFile(fileName, data, overwrite)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (v *Volume) CreateFile(name string, data []byte, overwrite bool) (*os.File, error) {
	vDir, err := v.getVolumeDir()
	if err != nil {
		return nil, err
	}
	fqName := path.Join(vDir.Name(), name)
	_, err = os.Stat(fqName)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error validating if file exists - %v", err)
		}
	} else if !overwrite {
		return nil, fmt.Errorf("file already exists - %v", err)
	}
	f, err := os.Create(fqName)
	if err != nil {
		return nil, fmt.Errorf("error creating file %s inside volume %s - %v", name, v.Name, err)
	}
	_, err = f.Write(data)
	if err != nil {
		return nil, fmt.Errorf("error writing to file %s inside volume %s - %v", name, v.Name, err)
	}
	return f, nil
}

func (v *Volume) DeleteFile(name string, recursive bool) error {
	vDir, err := v.getVolumeDir()
	if err != nil {
		return err
	}
	fqName := path.Join(vDir.Name(), name)
	if recursive {
		return os.RemoveAll(fqName)
	}
	return os.Remove(fqName)
}

type Port struct {
	Host     string
	HostIP   string
	Target   string
	Protocol string
}

type ContainerNetworkInfo struct {
	ID          string
	IPAddress   string
	IPPrefixLen int
	MacAddress  string
	Gateway     string
	Aliases     []string
}

type Image struct {
	Id         string
	Repository string
	Created    string
}

type Network struct {
	ID        string
	Name      string
	Subnets   []*Subnet
	Driver    string
	DNS       bool
	Internal  bool
	Labels    map[string]string
	Options   map[string]string
	CreatedAt string
}

type Subnet struct {
	Subnet  string
	Gateway string
}
