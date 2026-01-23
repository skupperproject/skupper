package common

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	corev1 "k8s.io/api/core/v1"
)

func CopySiteState(siteState *api.SiteState) *api.SiteState {
	// Preserving loaded state
	var activeSiteState = api.NewSiteState(siteState.IsBundle())
	siteState.Site.DeepCopyInto(activeSiteState.Site)
	activeSiteState.SiteId = siteState.SiteId
	activeSiteState.Listeners = copySiteStateMap(siteState.Listeners)
	activeSiteState.Connectors = copySiteStateMap(siteState.Connectors)
	activeSiteState.RouterAccesses = copySiteStateMap(siteState.RouterAccesses)
	activeSiteState.Claims = copySiteStateMap(siteState.Claims)
	activeSiteState.Links = copySiteStateMap(siteState.Links)
	activeSiteState.Grants = copySiteStateMap(siteState.Grants)
	activeSiteState.SecuredAccesses = copySiteStateMap(siteState.SecuredAccesses)
	activeSiteState.Certificates = copySiteStateMap(siteState.Certificates)
	activeSiteState.Secrets = copySiteStateMap(siteState.Secrets)
	activeSiteState.ConfigMaps = copySiteStateMap(siteState.ConfigMaps)
	return activeSiteState
}

func copySiteStateMap[T any](m map[string]T) map[string]T {
	if m == nil {
		return nil
	}
	newMap := make(map[string]T, len(m))
	for k, v := range m {
		var c any
		switch vv := any(v).(type) {
		case *v2alpha1.Listener:
			c = vv.DeepCopy()
		case *v2alpha1.Connector:
			c = vv.DeepCopy()
		case *v2alpha1.RouterAccess:
			c = vv.DeepCopy()
		case *v2alpha1.AccessGrant:
			c = vv.DeepCopy()
		case *v2alpha1.Link:
			c = vv.DeepCopy()
		case *v2alpha1.AccessToken:
			c = vv.DeepCopy()
		case *v2alpha1.Certificate:
			c = vv.DeepCopy()
		case *v2alpha1.SecuredAccess:
			c = vv.DeepCopy()
		case *corev1.Secret:
			c = vv.DeepCopy()
		}
		if c != nil {
			newMap[k] = c.(T)
		} else {
			newMap[k] = v
		}
	}
	return newMap
}

func CreateRouterAccess(siteState *api.SiteState) error {
	if !siteState.HasRouterAccess() {
		logger := NewLogger()
		logger.Debug("Creating skupper-local RouterAccess")
		name := fmt.Sprintf("skupper-local")
		var port = 5671
		var err error
		if !siteState.IsBundle() {
			port, err = utils.TcpPortNextFree(port)
			if err != nil {
				return err
			}
			logger.Debug("Free TCP port discovered", "port", port)
		}
		siteState.CreateRouterAccess(name, port)
	}
	return nil
}

func UpdateRouterAccess(siteState *api.SiteState, config *qdr.RouterConfig) error {
	if !siteState.HasRouterAccess() {
		logger := NewLogger()
		logger.Debug("Updating skupper-local RouterAccess")
		name := fmt.Sprintf("skupper-local")
		if config == nil {
			return fmt.Errorf("skupper-local RouterAccess cannot be created without a router config")
		}

		if config.Listeners == nil {
			return fmt.Errorf("skupper-local RouterAccess cannot be updated without listeners")
		}

		port := config.Listeners["skupper-local-normal"].Port
		siteState.CreateRouterAccess(name, int(port))
	}
	return nil
}
