package api

import (
	encodingjson "encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/site"
	"github.com/skupperproject/skupper/internal/version"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
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
	Claims          map[string]*v2alpha1.AccessToken
	Certificates    map[string]*v2alpha1.Certificate
	SecuredAccesses map[string]*v2alpha1.SecuredAccess
	Secrets         map[string]*corev1.Secret
	ConfigMaps      map[string]*corev1.ConfigMap
	bundle          bool
}

func NewSiteState(bundle bool) *SiteState {
	return &SiteState{
		Site:            &v2alpha1.Site{},
		Listeners:       make(map[string]*v2alpha1.Listener),
		Connectors:      make(map[string]*v2alpha1.Connector),
		RouterAccesses:  make(map[string]*v2alpha1.RouterAccess),
		Grants:          make(map[string]*v2alpha1.AccessGrant),
		Links:           make(map[string]*v2alpha1.Link),
		Claims:          make(map[string]*v2alpha1.AccessToken),
		Certificates:    make(map[string]*v2alpha1.Certificate),
		SecuredAccesses: make(map[string]*v2alpha1.SecuredAccess),
		Secrets:         make(map[string]*corev1.Secret),
		ConfigMaps:      make(map[string]*corev1.ConfigMap),
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
	return !s.Site.Spec.Edge
}

func (s *SiteState) HasRouterAccess() bool {
	for _, la := range s.RouterAccesses {
		for _, role := range la.Spec.Roles {
			if la.Name == "skupper-local" && role.Name == "normal" {
				return true
			}
		}
	}
	return false
}

func (s *SiteState) HasLinkAccess() bool {
	for _, la := range s.RouterAccesses {
		for _, role := range la.Spec.Roles {
			if role.Name == "edge" || role.Name == "inter-router" {
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
			if slices.Contains([]string{"edge", "inter-router"}, role.Name) {
				create = true
				break
			}
		}
		if !create {
			continue
		}
		hosts := linkAccess.Spec.SubjectAlternativeNames
		if linkAccess.Spec.BindHost != "" && !slices.Contains(hosts, linkAccess.Spec.BindHost) {
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
		siteLink := site.NewLink(name, path.Join(sslProfileBasePath, string(CertificatesPath)))
		link.SetConfigured(nil)
		siteLink.Update(link)
		linkMap[name] = siteLink
	}
	return linkMap
}

func (s *SiteState) bindings(sslProfileBasePath string) *site.Bindings {
	b := site.NewBindings(path.Join(sslProfileBasePath, string(CertificatesPath)))
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
		now := time.Now()
		routerName = fmt.Sprintf("%s-%d-%d", s.Site.Name, now.Unix(), now.Nanosecond())
	}
	routerConfig := qdr.InitialConfig(routerName, s.SiteId, version.Version, !s.IsInterior(), 3)
	routerConfig.AddAddress(qdr.Address{
		Prefix:       "mc",
		Distribution: "multicast",
	})

	routerConfig.SiteConfig = &qdr.SiteConfig{
		Name:      routerName,
		Namespace: s.GetNamespace(),
		Platform:  platform,
		Version:   version.Version,
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
		routerConfig.SiteConfig.Name += "-{{.SiteNameSuffix}}"
		routerConfig.SiteConfig.Namespace = "{{.Namespace}}"
		routerConfig.SiteConfig.Platform = "{{.Platform}}"
	}
	// LinkAccess
	s.linkAccessMap().DesiredConfig(nil, path.Join(sslProfileBasePath, string(CertificatesPath))).Apply(&routerConfig)
	// Link
	s.linkMap(sslProfileBasePath).Apply(&routerConfig)
	// Bindings
	s.bindings(sslProfileBasePath).Apply(&routerConfig)
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
	setNamespaceOnMap(s.ConfigMaps, namespace)
}

func (s *SiteState) UpdateStatus(networkStatus network.NetworkStatusInfo) {
	siteRecords := network.ExtractSiteRecords(networkStatus)
	if reflect.DeepEqual(s.Site.Status.Network, siteRecords) {
		return
	}
	s.Site.Status.Network = siteRecords
	s.Site.Status.SitesInNetwork = len(siteRecords)
	linkRecords := network.GetLinkRecordsForSite(s.SiteId, siteRecords)

	for _, linkRecord := range linkRecords {
		if link, ok := s.Links[linkRecord.Name]; ok {
			link.SetOperational(linkRecord.Operational, linkRecord.RemoteSiteId, linkRecord.RemoteSiteName)
		}
	}
	for linkName, existingLink := range s.Links {
		exists := slices.ContainsFunc(linkRecords, func(record v2alpha1.LinkRecord) bool {
			return record.Name == linkName
		})
		if !exists {
			existingLink.SetOperational(false, "", "")
		}
	}

	// updating listeners and connectors
	for _, listener := range s.Listeners {
		listener.SetHasMatchingConnector(network.HasMatchingPair(networkStatus, listener.Spec.RoutingKey))
	}
	for _, connector := range s.Connectors {
		connector.SetHasMatchingListener(network.HasMatchingPair(networkStatus, connector.Spec.RoutingKey))
	}
}

func marshal(outputDirectory, resourceType, resourceName string, resource interface{}) error {
	var err error
	err = os.MkdirAll(outputDirectory, 0755)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %w", outputDirectory, err)
	}
	fileName := path.Join(outputDirectory, fmt.Sprintf("%s-%s.yaml", resourceType, resourceName))
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", fileName, err)
	}
	yaml := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	err = yaml.Encode(resource.(runtime.Object), file)
	if err != nil {
		return fmt.Errorf("error marshalling resource %s-%s: %w", resourceType, resourceName, err)
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
	if siteState.Site != nil && siteState.Site.ObjectMeta.UID == "" {
		siteState.Site.ObjectMeta.UID = types.UID(siteState.SiteId)
	}
	if err = marshal(outputDirectory, "Site", siteState.Site.Name, siteState.Site); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "Listener", siteState.Listeners); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "Connector", siteState.Connectors); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "RouterAccess", siteState.RouterAccesses); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "Link", siteState.Links); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "AccessGrant", siteState.Grants); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "AccessToken", siteState.Claims); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "Certificate", siteState.Certificates); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "SecuredAccess", siteState.SecuredAccesses); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "Secret", siteState.Secrets); err != nil {
		return err
	}
	if err = marshalMap(outputDirectory, "ConfigMap", siteState.ConfigMaps); err != nil {
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
