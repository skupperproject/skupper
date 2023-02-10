package podman

import (
	"path"

	"github.com/skupperproject/skupper/pkg/config"
)

var (
	ConfigFile = path.Join(config.GetDataHome(), "podman.yaml")
)

type Config struct {
	Endpoint string `yaml:"endpoint"`
}

type configFileHandler struct {
	config *config.ConfigFileHandlerCommon
}

func (p *configFileHandler) GetConfig() (*Config, error) {
	err := p.config.Load()
	if err != nil {
		return nil, err
	}
	return p.config.GetData().(*Config), nil
}

func (p *configFileHandler) Save(config *Config) error {
	p.config.SetData(config)
	return p.config.Save()
}

func NewPodmanConfigFileHandler() *configFileHandler {
	c := &config.ConfigFileHandlerCommon{}
	c.SetFileName(ConfigFile)
	c.SetData(&Config{})
	p := &configFileHandler{config: c}
	return p
}
