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
