package views

import (
	"strings"

	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/api"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

const unknown = "unknown"

func Sites(entries []store.Entry) []api.SiteRecord {
	results := make([]api.SiteRecord, 0, len(entries))
	for _, e := range entries {
		site, ok := e.Record.(vanflow.SiteRecord)
		if !ok {
			continue
		}
		results = append(results, Site(site))
	}
	return results
}

func Site(site vanflow.SiteRecord) api.SiteRecord {
	s := defaultSite(site.ID)
	s.StartTime, s.EndTime = vanflowTimes(site.BaseRecord)
	s.NameSpace = site.Namespace

	if site.Name != nil {
		s.Name = *site.Name
	}
	if site.Provider != nil {
		s.Provider = *site.Provider
	}
	if site.Platform != nil {
		platform := *site.Platform
		switch {
		case strings.EqualFold(platform, string(api.Kubernetes)):
			s.Platform = api.Kubernetes
		case strings.EqualFold(platform, string(api.Docker)):
			s.Platform = api.Docker
		case strings.EqualFold(platform, string(api.Podman)):
			s.Platform = api.Podman
		}
	}
	if site.Version != nil {
		s.SiteVersion = *site.Version
	}
	return s
}

func defaultSite(id string) api.SiteRecord {
	return api.SiteRecord{
		Identity:    id,
		Name:        unknown,
		Platform:    api.Unknown,
		SiteVersion: unknown,
	}
}

func vanflowTimes(b vanflow.BaseRecord) (start, end uint64) {
	if b.StartTime != nil {
		start = uint64(b.StartTime.UnixMicro())
	}
	if b.EndTime != nil {
		start = uint64(b.EndTime.UnixMicro())
	}
	return
}
