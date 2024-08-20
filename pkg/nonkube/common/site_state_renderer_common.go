package common

import (
	"fmt"
	"maps"

	"github.com/skupperproject/skupper/pkg/nonkube/apis"
	"github.com/skupperproject/skupper/pkg/utils"
)

func CopySiteState(siteState *apis.SiteState) *apis.SiteState {
	// Preserving loaded state
	var activeSiteState = apis.NewSiteState(siteState.IsBundle())
	siteState.Site.DeepCopyInto(activeSiteState.Site)
	activeSiteState.SiteId = siteState.SiteId
	activeSiteState.Listeners = maps.Clone(siteState.Listeners)
	activeSiteState.Connectors = maps.Clone(siteState.Connectors)
	activeSiteState.RouterAccesses = maps.Clone(siteState.RouterAccesses)
	activeSiteState.Claims = maps.Clone(siteState.Claims)
	activeSiteState.Links = maps.Clone(siteState.Links)
	activeSiteState.Grants = maps.Clone(siteState.Grants)
	activeSiteState.SecuredAccesses = maps.Clone(siteState.SecuredAccesses)
	activeSiteState.Certificates = maps.Clone(siteState.Certificates)
	activeSiteState.Secrets = maps.Clone(siteState.Secrets)
	return activeSiteState
}

func CreateRouterAccess(siteState *apis.SiteState) error {
	if !siteState.HasRouterAccess() {
		name := fmt.Sprintf("skupper-local")
		var port = 5671
		var err error
		if !siteState.IsBundle() {
			port, err = utils.TcpPortNextFree(port)
			if err != nil {
				return err
			}
		}
		siteState.CreateRouterAccess(name, port)
	}
	return nil
}
