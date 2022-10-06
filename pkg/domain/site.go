package domain

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

var MinimumCompatibleVersion = "0.8.0"

type Site interface {
	GetName() string
	GetId() string
	GetVersion() string
	GetMode() string
	GetPlatform() string
	GetCertAuthorities() []types.CertAuthority
	SetCertAuthorities(cas []types.CertAuthority)
	GetCredentials() []types.Credential
	SetCredentials(credentials []types.Credential)
	GetIngressClasses() []string
	GetDeployments() []SkupperDeployment
	SetDeployments(deployments []SkupperDeployment)
}

type SiteHandler interface {
	Create(s Site) error
	Get() (Site, error)
	Delete() error
	Update() error
}

// SiteCommon base implementation of the Site interface
type SiteCommon struct {
	Name            string
	Id              string
	Version         string
	Mode            string
	Platform        string
	CertAuthorities []types.CertAuthority
	Credentials     []types.Credential
	Deployments     []SkupperDeployment
}

func (s *SiteCommon) GetCertAuthorities() []types.CertAuthority {
	return s.CertAuthorities
}

func (s *SiteCommon) SetCertAuthorities(cas []types.CertAuthority) {
	s.CertAuthorities = cas
}

func (s *SiteCommon) GetCredentials() []types.Credential {
	if s.Credentials == nil {
		s.Credentials = []types.Credential{}
	}
	return s.Credentials
}

func (s *SiteCommon) SetCredentials(credentials []types.Credential) {
	s.Credentials = credentials
}

func (s *SiteCommon) GetDeployments() []SkupperDeployment {
	return s.Deployments
}

func (s *SiteCommon) SetDeployments(deployments []SkupperDeployment) {
	s.Deployments = deployments
}

func (s *SiteCommon) GetName() string {
	return s.Name
}

func (s *SiteCommon) GetId() string {
	return s.Id
}

func (s *SiteCommon) GetVersion() string {
	return s.Version
}

func (s *SiteCommon) GetMode() string {
	return s.Mode
}

func (s *SiteCommon) IsEdge() bool {
	return s.Mode == qdr.ModeEdge
}

func (s *SiteCommon) ValidateMinimumRequirements() error {
	reqMsg := func(field string) error {
		return fmt.Errorf("%s cannot be empty", field)
	}
	if s.Name == "" {
		return reqMsg("name")
	}
	if s.Platform == "" {
		return reqMsg("platform")
	}
	if s.Mode == "" {
		return reqMsg("mode")
	}
	if s.Id == "" {
		s.Id = utils.RandomId(10)
	}
	return nil
}

func ConfigureSiteCredentials(site Site, ingressHosts ...string) {
	isEdge := site.GetMode() != string(types.TransportModeEdge)

	// CAs
	cas := []types.CertAuthority{}
	if len(site.GetCertAuthorities()) > 0 {
		cas = site.GetCertAuthorities()
	}
	cas = append(cas, types.CertAuthority{Name: types.LocalCaSecret})
	if isEdge {
		cas = append(cas, types.CertAuthority{Name: types.SiteCaSecret})
	}
	cas = append(cas, types.CertAuthority{Name: types.ServiceCaSecret})
	site.SetCertAuthorities(cas)

	// Certificates
	credentials := []types.Credential{}
	if len(site.GetCredentials()) > 0 {
		credentials = site.GetCredentials()
	}
	credentials = append(credentials, types.Credential{
		CA:          types.LocalCaSecret,
		Name:        types.LocalServerSecret,
		Subject:     types.LocalTransportServiceName,
		Hosts:       []string{types.LocalTransportServiceName},
		ConnectJson: false,
		Post:        false,
	})
	credentials = append(credentials, types.Credential{
		CA:          types.LocalCaSecret,
		Name:        types.LocalClientSecret,
		Subject:     types.LocalTransportServiceName,
		Hosts:       []string{},
		ConnectJson: true,
		Post:        false,
	})

	credentials = append(credentials, types.Credential{
		CA:          types.ServiceCaSecret,
		Name:        types.ServiceClientSecret,
		Hosts:       []string{},
		ConnectJson: false,
		Post:        false,
		Simple:      true,
	})

	if isEdge {
		hosts := []string{types.TransportServiceName}
		hosts = append(hosts, ingressHosts...)
		credentials = append(credentials, types.Credential{
			CA:          types.SiteCaSecret,
			Name:        types.SiteServerSecret,
			Subject:     types.TransportServiceName,
			Hosts:       hosts,
			ConnectJson: false,
		})
	}
	site.SetCredentials(credentials)
}

// VerifySiteCompatibility returns nil if current site version is compatible
// with the provided version, otherwise it returns a clear error.
func VerifySiteCompatibility(localSiteVersion, remoteSiteVersion string) error {
	if utils.LessRecentThanVersion(remoteSiteVersion, localSiteVersion) {
		if !utils.IsValidFor(remoteSiteVersion, MinimumCompatibleVersion) {
			return fmt.Errorf("minimum version required %s", MinimumCompatibleVersion)
		}
	}
	return nil
}
