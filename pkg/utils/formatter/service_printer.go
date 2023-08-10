package formatter

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/utils"
	"strings"
)
import "encoding/json"
import "gopkg.in/yaml.v2"

type ServiceStatusPrinter struct {
	OriginalData *list
	Services     []Service `json:"services,omitempty" yaml:"services,omitempty"`
}

type Service struct {
	Address    string              `json:"address,omitempty" yaml:"address,omitempty"`
	Protocol   string              `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Port       []string            `json:"port,omitempty" yaml:"port,omitempty"`
	Authorized string              `json:"authorized,omitempty" yaml:"authorized,omitempty"`
	Targets    []map[string]string `json:"targets,omitempty" yaml:"targets,omitempty"`
	Labels     []map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

func (p *ServiceStatusPrinter) PrintJsonFormat() (string, error) {
	p.ChangeFormat()

	if p.Services == nil {
		return "", fmt.Errorf("error before marshalling: empty list")
	}

	jsonData, err := json.MarshalIndent(p.Services, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error while marshalling: %v", err)
	}

	return string(jsonData), nil
}

func (p *ServiceStatusPrinter) PrintYamlFormat() (string, error) {
	p.ChangeFormat()

	if p.Services == nil {
		return "", fmt.Errorf("error before marshalling: empty list")
	}

	yamlData, err := yaml.Marshal(p.Services)
	if err != nil {
		return "", fmt.Errorf("error while marshalling: %v", err)
	}

	return string(yamlData), nil
}

func (p *ServiceStatusPrinter) ChangeFormat() {
	var printServices []Service

	for _, svc := range p.OriginalData.children {

		var address string
		var protocol string
		var ports []string
		var authorized string
		notAuthorizedSuffix := " - not authorized"

		item := svc.item

		if strings.Contains(svc.item, notAuthorizedSuffix) {
			authorized = "false"
			item = strings.TrimSuffix(item, notAuthorizedSuffix)
		}

		svcDetails := strings.Split(item, " ")

		for index, value := range svcDetails {
			switch index {
			case 0:
				address = value
			case 1:
				protocol = strings.TrimPrefix(value, "(")
			case 2:
				continue
			default:
				ports = append(ports, strings.TrimSuffix(value, ")"))
			}
		}

		pService := Service{
			Address:    address,
			Protocol:   protocol,
			Port:       ports,
			Authorized: authorized,
		}

		for _, child := range svc.children {

			switch child.item {
			case "Targets:":
				pService.Targets = childrenDataToMaps(child.children)
			case "Labels:":
				pService.Labels = childrenDataToMaps(child.Children())
			}
		}

		printServices = append(printServices, pService)

	}

	p.Services = printServices
}

func childrenDataToMaps(children []*list) []map[string]string {

	var dataList []map[string]string

	for _, child := range children {

		value := utils.LabelToMapWithSep(child.item, " ")

		dataList = append(dataList, value)

	}

	return dataList
}
