package common

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	activeSiteState.MultiKeyListeners = copySiteStateMap(siteState.MultiKeyListeners)
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
		case *v2alpha1.MultiKeyListener:
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

func EnableLinkAccess(siteState *api.SiteState, previousSiteState *api.SiteState) error {

	routerAccessName := siteState.Site.Name

	if _, exists := siteState.RouterAccesses[routerAccessName]; exists {
		return nil
	}

	// Bundles are installed on a different host, so use default ports instead of
	// allocating free ports from the machine generating the bundle.
	// Prefer an explicit RouterAccess in the bundle input for a specific ingress
	// host/IP; when generating automatically, use a generic bindHost and SAN list.
	interRouterPort := 55671
	edgePort := 45671
	var sanByDefault []string
	var bindHost string
	if siteState.IsBundle() {
		bindHost = "0.0.0.0"
		sanByDefault = []string{"127.0.0.1", "localhost", "0.0.0.0", "::"}
	} else {
		interRouterPort, edgePort = previousRouterAccessPorts(previousSiteState, routerAccessName)
		if interRouterPort == 0 || edgePort == 0 {
			var err error
			interRouterPort, edgePort, err = utils.AllocateRouterAccessPorts()
			if err != nil {
				return fmt.Errorf("failed to allocate ports for RouterAccess: %w", err)
			}
		}
		var err error
		sanByDefault, err = utils.GetSansByDefault()
		if err != nil {
			return fmt.Errorf("Error getting SANs by default: %s", err)
		}
	}

	siteState.RouterAccesses[routerAccessName] = &v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      routerAccessName,
			Namespace: siteState.GetNamespace(),
		},
		Spec: v2alpha1.RouterAccessSpec{
			Roles: []v2alpha1.RouterAccessRole{
				{
					Name: "inter-router",
					Port: interRouterPort,
				},
				{
					Name: "edge",
					Port: edgePort,
				},
			},
			BindHost:                bindHost,
			SubjectAlternativeNames: sanByDefault,
		},
	}

	return nil
}

func previousRouterAccessPorts(previousSiteState *api.SiteState, routerName string) (interRouterPort int, edgePort int) {
	if previousSiteState == nil {
		return 0, 0
	}
	previous, ok := previousSiteState.RouterAccesses[routerName]
	if !ok || previous == nil {
		return 0, 0
	}
	if role := previous.FindRole("inter-router"); role != nil {
		interRouterPort = role.Port
	}
	if role := previous.FindRole("edge"); role != nil {
		edgePort = role.Port
	}
	return interRouterPort, edgePort
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

func RecoverRouterAccess(siteState *api.SiteState, config *qdr.RouterConfig) error {
	if !siteState.HasRouterAccess() {
		logger := NewLogger()
		logger.Debug("Recovering skupper-local RouterAccess")
		name := fmt.Sprintf("skupper-local")
		if config == nil {
			return fmt.Errorf("skupper-local RouterAccess cannot be recovered without a router config")
		}

		if config.Listeners == nil {
			return fmt.Errorf("skupper-local RouterAccess cannot be recovered without listeners")
		}

		var port int
		if _, ok := config.Listeners["skupper-local-normal"]; ok {
			port = int(config.Listeners["skupper-local-normal"].Port)
			siteState.CreateRouterAccess(name, port)
		} else {
			return fmt.Errorf("skupper-local-normal listener does not exists in the router config.")
		}

	}
	return nil
}
