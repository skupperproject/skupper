package podman

import (
	"path"

	"github.com/skupperproject/skupper/pkg/config"
)

var (
	PodmanConfigFile = path.Join(config.GetDataHome(), "podman.yaml")
)

type PodmanConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type podmanConfigFileHandler struct {
	config *config.ConfigFileHandlerCommon
}

func (p *podmanConfigFileHandler) GetConfig() (*PodmanConfig, error) {
	err := p.config.Load()
	if err != nil {
		return nil, err
	}
	return p.config.GetData().(*PodmanConfig), nil
}

func (p *podmanConfigFileHandler) Save(config *PodmanConfig) error {
	p.config.SetData(config)
	return p.config.Save()
}

func NewPodmanConfigFileHandler() *podmanConfigFileHandler {
	c := &config.ConfigFileHandlerCommon{}
	c.SetFileName(PodmanConfigFile)
	c.SetData(&PodmanConfig{})
	p := &podmanConfigFileHandler{config: c}
	return p
}
