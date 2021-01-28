package data

import (
	"strings"
)

type Service struct {
	Address  string          `json:"address"`
	Protocol string          `json:"protocol"`
	Targets  []ServiceTarget `json:"targets"`
}

type ServiceTarget struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	SiteId string `json:"site_id"`
}

func (s *Service) AddTarget(name string, host string, siteId string, mapping NameMapping) {
	target := ServiceTarget{
		Name:   mapping.Lookup(host),
		Target: strings.Split(name, "@")[0],
		SiteId: siteId,
	}
	s.Targets = append(s.Targets, target)
}
