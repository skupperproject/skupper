package api

import (
	encodingjson "encoding/json"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

type StaticSiteStateRenderer interface {
	Render(state *SiteState, reload bool) error
}

type SiteState struct {
	SiteId          string
	Site            *v2alpha1.Site
	Listeners       map[string]*v2alpha1.Listener
	Connectors      map[string]*v2alpha1.Connector
	RouterAccesses  map[string]*v2alpha1.RouterAccess
	Grants          map[string]*v2alpha1.AccessGrant
	Links           map[string]*v2alpha1.Link
	Secrets         map[string]*corev1.Secret
	Claims          map[string]*v2alpha1.AccessToken
	Certificates    map[string]*v2alpha1.Certificate
	SecuredAccesses map[string]*v2alpha1.SecuredAccess
	bundle          bool
}

func NewSiteState(bundle bool) *SiteState {
	return &SiteState{
		Site:            &v2alpha1.Site{},
		Listeners:       make(map[string]*v2alpha1.Listener),
		Connectors:      make(map[string]*v2alpha1.Connector),
		RouterAccesses:  map[string]*v2alpha1.RouterAccess{},
		Grants:          make(map[string]*v2alpha1.AccessGrant),
		Links:           make(map[string]*v2alpha1.Link),
		Secrets:         make(map[string]*corev1.Secret),
		Claims:          make(map[string]*v2alpha1.AccessToken),
		Certificates:    map[string]*v2alpha1.Certificate{},
		SecuredAccesses: map[string]*v2alpha1.SecuredAccess{},
		bundle:          bundle,
	}
}

func (s *SiteState) GetNamespace() string {
	ns := s.Site.GetNamespace()
	if ns == "" {
		return "default"
	}
	return ns
}

func (s *SiteState) IsBundle() bool {
	return s.bundle
}

func (s *SiteState) IsInterior() bool {
	return s.Site.Spec.RouterMode != "edge"
}

func (s *SiteState) HasRouterAccess() bool {
	for _, la := range s.RouterAccesses {
		for _, role := range la.Spec.Roles {
			if role.Name == "normal" {
				return true
			}
		}
	}
	return false
}

func (s *SiteState) CreateRouterAccess(name string, port int) {
	tlsCaName := fmt.Sprintf("%s-ca", name)
	tlsServerName := fmt.Sprintf("%s-server", name)
	tlsClientName := fmt.Sprintf("%s-client", name)
	s.RouterAccesses[name] = &v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.GetNamespace(),
		},
		Spec: v2alpha1.RouterAccessSpec{
			Roles: []v2alpha1.RouterAccessRole{
				{
					Name: "normal",
					Port: port,
				},
			},
			BindHost:       "127.0.0.1",
			TlsCredentials: tlsServerName,
			Issuer:         tlsCaName,
		},
	}
	s.RouterAccesses[name].SetConfigured(nil)
	s.Certificates[tlsCaName] = s.newCertificate(tlsCaName, &v2alpha1.CertificateSpec{
		Subject: tlsCaName,
		Hosts:   []string{"127.0.0.1", "localhost"},
		Signing: true,
	})
	s.Certificates[tlsServerName] = s.newCertificate(tlsServerName, &v2alpha1.CertificateSpec{
		Subject: "127.0.0.1",
		Hosts:   []string{"127.0.0.1", "localhost"},
		Ca:      tlsCaName,
		Server:  true,
	})
	s.Certificates[tlsClientName] = s.newCertificate(tlsClientName, &v2alpha1.CertificateSpec{
		Subject: "127.0.0.1",
		Hosts:   []string{"127.0.0.1", "localhost"},
		Ca:      tlsCaName,
		Client:  true,
	})
}

func (s *SiteState) CreateLinkAccessesCertificates() {
	caName := fmt.Sprintf("skupper-site-ca")
	s.Certificates[caName] = s.newCertificate(caName, &v2alpha1.CertificateSpec{
		Subject: caName,
		Signing: true,
	})

	for name, linkAccess := range s.RouterAccesses {
		create := false
		for _, role := range linkAccess.Spec.Roles {
			if utils.StringSliceContains([]string{"edge", "inter-router"}, role.Name) {
				create = true
				break
			}
		}
		if !create {
			continue
		}
		hosts := linkAccess.Spec.SubjectAlternativeNames
		if linkAccess.Spec.BindHost != "" && !utils.StringSliceContains(hosts, linkAccess.Spec.BindHost) {
			hosts = append(hosts, linkAccess.Spec.BindHost)
		}
		linkAccessCaName := caName
		if linkAccess.Spec.Issuer != "" {
			linkAccessCaName = linkAccess.Spec.Issuer
		}
		if linkAccessCaName != caName {
			s.Certificates[linkAccessCaName] = s.newCertificate(linkAccessCaName, &v2alpha1.CertificateSpec{
				Subject: linkAccessCaName,
				Signing: true,
			})
		}
		certName := name
		if linkAccess.Spec.TlsCredentials != "" {
			certName = linkAccess.Spec.TlsCredentials
		} else {
			linkAccess.Spec.TlsCredentials = name
		}
		s.Certificates[certName] = s.newCertificate(certName, &v2alpha1.CertificateSpec{
			Ca:      linkAccessCaName,
			Subject: name,
			Hosts:   hosts,
			Server:  true,
		})
		clientCertificateName := fmt.Sprintf("client-%s", certName)
		s.Certificates[clientCertificateName] = s.newCertificate(clientCertificateName, &v2alpha1.CertificateSpec{
			Ca:      linkAccessCaName,
			Subject: clientCertificateName,
			Client:  true,
		})
		linkAccess.SetConfigured(nil)
	}

}

func (s *SiteState) CreateBridgeCertificates() {
	caName := fmt.Sprintf("skupper-service-ca")
	s.Certificates[caName] = s.newCertificate(caName, &v2alpha1.CertificateSpec{
		Subject: caName,
		Signing: true,
	})
	for _, listener := range s.Listeners {
		if listener.Spec.TlsCredentials != "" {
			s.Certificates[listener.Spec.TlsCredentials] = s.newCertificate(listener.Spec.TlsCredentials, &v2alpha1.CertificateSpec{
				Ca:      caName,
				Subject: listener.Spec.Host,
				Hosts:   []string{listener.Spec.Host},
				Server:  true,
			})
		}
	}
	for _, connector := range s.Connectors {
		if connector.Spec.TlsCredentials != "" {
			s.Certificates[connector.Spec.TlsCredentials] = s.newCertificate(connector.Spec.TlsCredentials, &v2alpha1.CertificateSpec{
				Ca:      caName,
				Subject: connector.Spec.Host,
				Hosts:   []string{connector.Spec.Host},
				Server:  true,
			})
		}
	}
}

func (s *SiteState) newCertificate(name string, spec *v2alpha1.CertificateSpec) *v2alpha1.Certificate {
	return &v2alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.GetNamespace(),
		},
		Spec: *spec,
		Status: v2alpha1.CertificateStatus{
			Status: v2alpha1.Status{
				Message: v2alpha1.STATUS_OK,
			},
		},
	}
}

func (s *SiteState) linkAccessMap() site.RouterAccessMap {
	linkAccessMap := site.RouterAccessMap{}
	for name, linkAccess := range s.RouterAccesses {
		linkAccessMap[name] = linkAccess
	}
	return linkAccessMap
}
func (s *SiteState) linkMap(sslProfileBasePath string) site.LinkMap {
	linkMap := site.LinkMap{}
	for name, link := range s.Links {
		siteLink := site.NewLink(name, path.Join(sslProfileBasePath, "certificates/link"))
		link.SetConfigured(nil)
		siteLink.Update(link)
		linkMap[name] = siteLink
	}
	return linkMap
}

func (s *SiteState) bindings() *site.Bindings {
	b := site.NewBindings()
	for name, connector := range s.Connectors {
		connector.SetConfigured(nil)
		_ = b.UpdateConnector(name, connector)
	}
	for name, listener := range s.Listeners {
		listener.SetConfigured(nil)
		_ = b.UpdateListener(name, listener)
	}
	return b
}

func (s *SiteState) ToRouterConfig(sslProfileBasePath string, platform string) qdr.RouterConfig {
	if s.SiteId == "" {
		s.SiteId = uuid.New().String()
	}
	var routerName = s.Site.Name
	if !s.bundle {
		routerName = fmt.Sprintf("%s-%d", s.Site.Name, time.Now().Unix())
	}
	routerConfig := qdr.InitialConfig(routerName, s.SiteId, version.Version, !s.IsInterior(), 3)
	routerConfig.SiteConfig = &qdr.SiteConfig{
		Name:      routerName,
		Namespace: s.GetNamespace(),
		Platform:  platform,
	}

	// override metadata
	if s.bundle {
		routerConfig.Metadata.Id += "-{{.SiteNameSuffix}}"
		metadata := qdr.SiteMetadata{
			Id:       "{{.SiteId}}",
			Version:  version.Version,
			Platform: "{{.Platform}}",
		}
		metadataJson, _ := encodingjson.Marshal(metadata)
		routerConfig.Metadata.Metadata = string(metadataJson)
		routerConfig.SiteConfig.Namespace = "{{.Namespace}}"
		routerConfig.SiteConfig.Platform = "{{.Platform}}"
	}
	// LinkAccess
	s.linkAccessMap().DesiredConfig(nil, path.Join(sslProfileBasePath, "certificates/server")).Apply(&routerConfig)
	// Link
	s.linkMap(sslProfileBasePath).Apply(&routerConfig)
	// Bindings
	s.bindings().Apply(&routerConfig)
	// Log (static for now) TODO use site specific options to configure logging
	routerConfig.SetLogLevel("ROUTER_CORE", "error+")

	return routerConfig
}

func setNamespaceOnMap[T metav1.Object](objMap map[string]T, namespace string) {
	for _, obj := range objMap {
		obj.SetNamespace(namespace)
	}
}

func (s *SiteState) SetNamespace(namespace string) {
	if namespace == "" {
		namespace = "default"
	}
	// equals
	if s.GetNamespace() == namespace {
		return
	}
	s.Site.SetNamespace(namespace)
	setNamespaceOnMap(s.Listeners, namespace)
	setNamespaceOnMap(s.Connectors, namespace)
	setNamespaceOnMap(s.RouterAccesses, namespace)
	setNamespaceOnMap(s.Grants, namespace)
	setNamespaceOnMap(s.Links, namespace)
	setNamespaceOnMap(s.Secrets, namespace)
	setNamespaceOnMap(s.Claims, namespace)
	setNamespaceOnMap(s.Certificates, namespace)
	setNamespaceOnMap(s.SecuredAccesses, namespace)
}

func marshal(outputDirectory, resourceType, resourceName string, resource interface{}) error {
	var err error
	writeDirectory := path.Join(outputDirectory, resourceType)
	err = os.MkdirAll(writeDirectory, 0755)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %w", writeDirectory, err)
	}
	fileName := path.Join(writeDirectory, fmt.Sprintf("%s.yaml", resourceName))
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", fileName, err)
	}
	yaml := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	err = yaml.Encode(resource.(runtime.Object), file)
	if err != nil {
		return fmt.Errorf("error marshalling resource %s: %w", resourceName, err)
	}
	return nil
}

func marshalMap[V any](outputDirectory, resourceType string, resourceMap map[string]V) error {
	var err error
	for resourceName, resource := range resourceMap {
		if err = marshal(outputDirectory, resourceType, resourceName, resource); err != nil {
			return err
		}
	}
	return nil
}

func MarshalSiteState(siteState SiteState, outputDirectory string) error {
	var err error
	if err = marshal(outputDirectory, "site", siteState.Site.Name, siteState.Site); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "listeners", siteState.Listeners); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "connectors", siteState.Connectors); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "routerAccesses", siteState.RouterAccesses); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "links", siteState.Links); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "grants", siteState.Grants); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "claims", siteState.Claims); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "certificates", siteState.Certificates); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "securedAccesses", siteState.SecuredAccesses); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "secrets", siteState.Secrets); err != nil {
		return err
	}
	return nil
}

type SiteStateLoader interface {
	Load() (*SiteState, error)
}

type SiteStateValidator interface {
	Validate(site *SiteState) error
}
