package formatter

import (
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"gopkg.in/yaml.v2"
)

type NetworkStatusPrinter struct {
	OriginalData []*types.SiteInfo
}

func (p *NetworkStatusPrinter) PrintJsonFormat() (string, error) {
	jsonData, err := json.MarshalIndent(p.OriginalData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error while marshalling: %v", err)
	}

	return string(jsonData), nil
}

func (p *NetworkStatusPrinter) PrintYamlFormat() (string, error) {
	yamlData, err := yaml.Marshal(p.OriginalData)
	if err != nil {
		return "", fmt.Errorf("error while marshalling: %v", err)
	}

	return string(yamlData), nil
}

func (p *NetworkStatusPrinter) ChangeFormat() {

}
