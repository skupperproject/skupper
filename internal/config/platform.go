package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"gopkg.in/yaml.v3"
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

	// Creating the base dir
	baseDir := filepath.Dir(PlatformConfigFile)
	if _, err := os.Stat(baseDir); err != nil {
		if err = os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("unable to create base directory %s - %q", baseDir, err)
		}
	}
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
	data, err := os.ReadFile(PlatformConfigFile)
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
	baseDir := filepath.Dir(l.GetFilename())
	if _, err := os.Stat(baseDir); err != nil {
		if err = os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("unable to create base directory %s - %q", baseDir, err)
		}
	}
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
	data, err := os.ReadFile(l.GetFilename())
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
	PlatformConfigFile = path.Join(getDataHome(), "platform.yaml")
)

var (
	Platform string
)

// GetPlatform returns the runtime platform defined,
// where the lookup goes through the following sequence:
// - Platform variable,
// - SKUPPER_PLATFORM environment variable
// - Static platform defined by skupper switch
// - Default platform "kubernetes" otherwise.
// In case the defined platform is invalid, "kubernetes"
// will be returned.
func GetPlatform() types.Platform {
	p := &PlatformInfo{}
	var platform types.Platform
	_ = p.Load()
	for i, arg := range os.Args {
		if slices.Contains([]string{"--platform", "-p"}, arg) && i+1 < len(os.Args) {
			platformArg := os.Args[i+1]
			platform = types.Platform(platformArg)
			break
		} else if strings.HasPrefix(arg, "--platform=") || strings.HasPrefix(arg, "-p=") {
			platformArg := strings.Split(arg, "=")[1]
			platform = types.Platform(platformArg)
			break
		}
	}
	if platform == "" {
		platform = types.Platform(utils.DefaultStr(Platform,
			os.Getenv(types.ENV_PLATFORM),
			string(p.Current),
			string(types.PlatformKubernetes)))
	}
	switch platform {
	case types.PlatformPodman:
		return types.PlatformPodman
	case types.PlatformDocker:
		return types.PlatformDocker
	case types.PlatformSystemd:
		return types.PlatformSystemd
	default:
		return types.PlatformKubernetes
	}
}

func getDataHome() string {
	if os.Getuid() == 0 {
		return "/var/lib/skupper"
	}
	dataHome, ok := os.LookupEnv("XDG_DATA_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		dataHome = homeDir + "/.local/share"
	}
	return path.Join(dataHome, "skupper")
}
