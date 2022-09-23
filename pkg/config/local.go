package config

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	yaml "gopkg.in/yaml.v3"
)

type ConfigFileHandler interface {
	GetFilename() string
	SetFileName(name string)
	Save() error
	Load() error
	GetData() interface{}
	SetData(data interface{})
}

type ConfigFileHandlerCommon struct {
	filename string
	data     interface{}
}

func (l *ConfigFileHandlerCommon) GetFilename() string {
	return l.filename
}

func (l *ConfigFileHandlerCommon) SetFileName(name string) {
	l.filename = name
}

func (l *ConfigFileHandlerCommon) Save() error {
	f, err := os.Create(l.GetFilename())
	if err != nil {
		return fmt.Errorf("error creating file %s: %v", l.GetFilename(), err)
	}
	defer f.Close()
	e := yaml.NewEncoder(f)
	if err = e.Encode(l.GetData()); err != nil {
		return fmt.Errorf("error saving file: %s: %v", l.GetFilename(), err)
	}
	return nil
}

func (l *ConfigFileHandlerCommon) Load() error {
	data, err := ioutil.ReadFile(l.GetFilename())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error loading %s: %v", l.GetFilename(), err)
	}
	if data != nil {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		if err = decoder.Decode(l.data); err != nil && err != io.EOF {
			return fmt.Errorf("error decoding %s: %v", l.GetFilename(), err)
		}
	}
	return nil
}

func (l *ConfigFileHandlerCommon) GetData() interface{} {
	return l.data
}

func (l *ConfigFileHandlerCommon) SetData(data interface{}) {
	l.data = data
}

var (
	PlatformConfigFile = path.Join(GetDataHome(), "platform.yaml")
)

type PlatformInfo struct {
	Current  types.Platform `yaml:"current"`
	Previous types.Platform `yaml:"previous"`
}

func (p *PlatformInfo) Update(platform types.Platform) error {
	if err := p.Load(); err != nil {
		return err
	}
	if p.Current == "" {
		p.Current = platform
	}
	p.Previous = p.Current
	p.Current = platform

	f, err := os.Create(PlatformConfigFile)
	if err != nil {
		return fmt.Errorf("error creating file %s: %v", PlatformConfigFile, err)
	}
	defer f.Close()
	e := yaml.NewEncoder(f)
	if err = e.Encode(p); err != nil {
		return fmt.Errorf("error saving file: %s: %v", PlatformConfigFile, err)
	}
	return nil
}

func (p *PlatformInfo) Load() error {
	data, err := ioutil.ReadFile(PlatformConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error loading %s: %v", PlatformConfigFile, err)
	}
	if data != nil {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		if err = decoder.Decode(p); err != nil && err != io.EOF {
			return fmt.Errorf("error decoding %s: %v", PlatformConfigFile, err)
		}
	}
	return nil
}

var (
	Platform string
)

func GetPlatform() types.Platform {
	p := &PlatformInfo{}
	_ = p.Load()
	return types.Platform(utils.DefaultStr(Platform,
		os.Getenv(types.ENV_PLATFORM),
		string(p.Current),
		string(types.PlatformKubernetes)))
}

func GetDataHome() string {
	dataHome, ok := os.LookupEnv("XDG_DATA_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		dataHome = homeDir + "/.local/share"
	}
	return path.Join(dataHome, "skupper")
}

func GetConfigHome() string {
	configHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		return homeDir + "/.config"
	} else {
		return configHome
	}
}

func GetRuntimeDir() string {
	runtimeDir, ok := os.LookupEnv("XDG_RUNTIME_DIR")
	if !ok {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return runtimeDir
}
