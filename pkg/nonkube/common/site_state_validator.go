package common

import (
	"fmt"
	"net"
	"regexp"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/apis"
	"github.com/skupperproject/skupper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

var (
	validLinkAccessRoles = []string{"edge", "inter-router"}
	rfc1123Regex         = regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")
	hostnameRfc1123Regex = regexp.MustCompile(`^[a-z0-9]+([-.]{1}[a-z0-9]+)*$`)
)

const (
	rfc1123Error = `a lowercase RFC 1123 name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')`
)

type SiteStateValidator struct {
}

// Validate provides a common validation for non-kubernetes sites
// which do not benefit from the Kubernetes API. The goal is not
// to validate the site state against the spec (CRD), but to a more
// basic level, like to ensure that mandatory fields for each resource are
// populated and users will be able to operate the non-k8s site.
func (s *SiteStateValidator) Validate(siteState *apis.SiteState) error {
	var err error
	if err = s.validateSite(siteState.Site); err != nil {
		return err
	}
	if err = s.validateRouterAccesses(siteState.RouterAccesses); err != nil {
		return err
	}
	if err = s.validateLinks(siteState.Links, siteState.Secrets); err != nil {
		return err
	}
	if err = s.validateClaims(siteState.Claims); err != nil {
		return err
	}
	if err = s.validateGrants(siteState.Grants); err != nil {
		return err
	}
	if err = s.validateListeners(siteState.Listeners); err != nil {
		return err
	}
	if err = s.validateConnectors(siteState.Connectors); err != nil {
		return err
	}

	return nil
}

func (s *SiteStateValidator) validateSite(site *v1alpha1.Site) error {
	if err := ValidateName(site.Name); err != nil {
		return fmt.Errorf("invalid site name: %w", err)
	}
	return nil
}

func (s *SiteStateValidator) validateRouterAccesses(routerAccesses map[string]*v1alpha1.RouterAccess) error {
	for _, routerAccess := range routerAccesses {
		if err := ValidateName(routerAccess.Name); err != nil {
			return fmt.Errorf("invalid router access name: %w", err)
		}
		if routerAccess.Spec.TlsCredentials != "" {
			if err := ValidateName(routerAccess.Spec.TlsCredentials); err != nil {
				return fmt.Errorf("invalid router access tls credentials: %w", err)
			}
		}
		if len(routerAccess.Spec.Roles) == 0 {
			return fmt.Errorf("invalid router access: roles are required")
		}
		for _, role := range routerAccess.Spec.Roles {
			if !utils.StringSliceContains(validLinkAccessRoles, role.Name) {
				return fmt.Errorf("invalid router access: %s - invalid role: %s (valid roles: %s)",
					routerAccess.Name, role.Name, validLinkAccessRoles)
			}
		}
	}
	return nil
}

func (s *SiteStateValidator) validateLinks(links map[string]*v1alpha1.Link, secrets map[string]*corev1.Secret) error {
	if links == nil || len(links) == 0 {
		return nil
	}
	if secrets == nil || len(secrets) == 0 {
		return fmt.Errorf("unable to process links (no secrets found)")
	}
	for linkName, link := range links {
		if err := ValidateName(link.Name); err != nil {
			return fmt.Errorf("invalid link name: %w", err)
		}
		secretName := link.Spec.TlsCredentials
		if _, ok := secrets[secretName]; !ok {
			return fmt.Errorf("invalid link %q - secret %q not found", linkName, secretName)
		}
	}
	return nil
}

func (s *SiteStateValidator) validateClaims(claims map[string]*v1alpha1.AccessToken) error {
	for _, claim := range claims {
		if err := ValidateName(claim.Name); err != nil {
			return fmt.Errorf("invalid access token name: %w", err)
		}
	}
	return nil
}

func (s *SiteStateValidator) validateGrants(grants map[string]*v1alpha1.AccessGrant) error {
	for _, grant := range grants {
		if err := ValidateName(grant.Name); err != nil {
			return fmt.Errorf("invalid grant name: %w", err)
		}
	}
	return nil
}

func (s *SiteStateValidator) validateListeners(listeners map[string]*v1alpha1.Listener) error {
	hostPorts := map[string][]int{}
	for name, listener := range listeners {
		if err := ValidateName(listener.Name); err != nil {
			return fmt.Errorf("invalid listener name: %w", err)
		}
		if listener.Spec.Host == "" || listener.Spec.Port == 0 {
			return fmt.Errorf("host and port are required")
		}
		if ip := net.ParseIP(listener.Spec.Host); ip == nil {
			return fmt.Errorf("invalid listener host: %s - a valid IP address is expected", listener.Spec.Host)
		}

		if utils.IntSliceContains(hostPorts[listener.Spec.Host], listener.Spec.Port) {
			return fmt.Errorf("port %d is already mapped for host %q (listener: %q)", listener.Spec.Port, listener.Spec.Host, name)
		}
		hostPorts[listener.Spec.Host] = append(hostPorts[listener.Spec.Host], listener.Spec.Port)
	}
	return nil
}

func (s *SiteStateValidator) validateConnectors(connectors map[string]*v1alpha1.Connector) error {
	for _, connector := range connectors {
		if err := ValidateName(connector.Name); err != nil {
			return fmt.Errorf("invalid connector name: %w", err)
		}
		if connector.Spec.Host == "" || connector.Spec.Port == 0 {
			return fmt.Errorf("connector host and port are required")
		}
		ip := net.ParseIP(connector.Spec.Host)
		validHostname := hostnameRfc1123Regex.MatchString(connector.Spec.Host)
		if ip == nil && !validHostname {
			return fmt.Errorf("invalid connector host: %s - a valid IP address or hostname is expected", connector.Spec.Host)
		}
	}
	return nil
}

func ValidateName(name string) error {
	if !rfc1123Regex.MatchString(name) {
		return fmt.Errorf("invalid name %q: %s", name, rfc1123Error)
	}
	return nil
}
