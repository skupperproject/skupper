package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SkupperClusterPolicy defines optional cluster level policies
type SkupperClusterPolicy struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          SkupperClusterPolicySpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SkupperClusterPolicyList contains a List of SkupperClusterPolicy
type SkupperClusterPolicyList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []SkupperClusterPolicy `json:"items"`
}

type SkupperClusterPolicySpec struct {
	Namespaces                    []string `json:"namespaces"`
	AllowIncomingLinks            bool     `json:"allowIncomingLinks"`
	AllowedOutgoingLinksHostnames []string `json:"allowedOutgoingLinksHostnames"`
	AllowedExposedResources       []string `json:"allowedExposedResources"`
	AllowedServices               []string `json:"allowedServices"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Site defines the location and configuration of a skupper site
type Site struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          SiteSpec   `json:"spec,omitempty"`
	Status        SiteStatus `json:"status,omitempty"`
}

func (s *Site) GetSiteId() string {
	return string(s.ObjectMeta.UID)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiteList contains a List of Site instances
type SiteList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Site `json:"items"`
}

type SiteSpec struct {
	ServiceAccount string            `json:"serviceAccount,omitempty"`
	LinkAccess     string            `json:"linkAccess,omitempty"`
	DefaultIssuer  string            `json:"defaultIssuer,omitempty"`
	RouterMode     string            `json:"routerMode,omitempty"`
	HA             bool              `json:"ha,omitempty"`
	Settings       map[string]string `json:"settings,omitempty"`
}

func (s *SiteSpec) GetServiceAccount() string {
	if s.ServiceAccount == "" {
		return "skupper-router"
	}
	return s.ServiceAccount
}

func (s *SiteSpec) GetRouterLogging() string {
	if value, ok := s.Settings["router-logging"]; ok {
		return value
	}
	return ""
}

func (s *SiteSpec) GetRouterDataConnectionCount() string {
	if value, ok := s.Settings["router-data-connection-count"]; ok {
		return value
	}
	return ""
}

type Status struct {
	Active        bool   `json:"active"`
	StatusMessage string `json:"status,omitempty"`
}

const STATUS_OK = "OK"

func (s *Status) SetStatus(active bool, message string) bool {
	if s.Active != active || s.StatusMessage != message {
		s.Active = active
		s.StatusMessage = message
		return true
	}
	return false
}

type SiteStatus struct {
	Status            `json:",inline"`
	Endpoints         []Endpoint   `json:"endpoints,omitempty"`
	SitesInNetwork    int          `json:"sitesInNetwork,omitempty"`
	ServicesInNetwork int          `json:"servicesInNetwork,omitempty"`
	Network           []SiteRecord `json:"network,omitempty"`
	DefaultIssuer     string       `json:"defaultIssuer,omitempty"`
}

type Endpoint struct {
	Name  string `json:"name,omitempty"`
	Host  string `json:"host,omitempty"`
	Port  string `json:"port,omitempty"`
	Group string `json:"group,omitempty"`
}

func (a *Endpoint) MatchHostPort(b *Endpoint) bool {
	return a.Host == b.Host && a.Port == b.Port
}

func (a *Endpoint) Url() string {
	return fmt.Sprintf("%s:%s", a.Host, a.Port)
}

type SiteRecord struct {
	Id        string          `json:"id"`
	Name      string          `json:"name"`
	Namespace string          `json:"namespace,omitempty"`
	Platform  string          `json:"platform,omitempty"`
	Version   string          `json:"version,omitempty"`
	Links     []string        `json:"links,omitempty"`
	Services  []ServiceRecord `json:"services,omitempty"`
}

type ServiceRecord struct {
	RoutingKey string   `json:"routingKey"`
	Connectors []string `json:"connectors"`
	Listeners  []string `json:"listeners"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Listener struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          ListenerSpec `json:"spec,omitempty"`
	Status        Status       `json:"status,omitempty"`
}


func (l *Listener) Protocol() corev1.Protocol {
	if l.Spec.Type == "udp" {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ListenerList contains a List of Listener instances
type ListenerList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Listener `json:"items"`
}

type ListenerSpec struct {
	RoutingKey     string `json:"routingKey"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	TlsCredentials string `json:"tlsCredentials,omitempty"`
	Type           string `json:"type,omitempty"`
}

type ServicePort struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Connector struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          ConnectorSpec `json:"spec,omitempty"`
	Status        Status        `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConnectorList contains a List of Connector instances
type ConnectorList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Connector `json:"items"`
}

type ConnectorSpec struct {
	RoutingKey      string `json:"routingKey"`
	Host            string `json:"host,omitempty"`
	Selector        string `json:"selector,omitempty"`
	Port            int    `json:"port"`
	TlsCredentials  string `json:"tlsCredentials,omitempty"`
	Type            string `json:"type,omitempty"`
	IncludeNotReady bool   `json:"includeNotReady,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Link struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          LinkSpec   `json:"spec,omitempty"`
	Status        LinkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinkList contains a List of Link instances
type LinkList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Link `json:"items"`
}

type LinkSpec struct {
	Endpoints      []Endpoint `json:"endpoints,omitempty"`
	TlsCredentials string     `json:"tlsCredentials,omitempty"`
	Cost           int        `json:"cost,omitempty"`
	NoClientAuth   bool       `json:"noClientAuth,omitempty"`
}

func (s *LinkSpec) GetEndpointForRole(name string) (Endpoint, bool) {
	for _, endpoint := range s.Endpoints {
		if endpoint.Name == name {
			return endpoint, true
		}
	}
	return Endpoint{}, false
}

type LinkStatus struct {
	Status     `json:",inline"`
	Configured bool `json:"configured,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AccessToken struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          AccessTokenSpec   `json:"spec,omitempty"`
	Status        AccessTokenStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessTokenList contains a List of AccessToken instances
type AccessTokenList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []AccessToken `json:"items"`
}

type AccessTokenSpec struct {
	Url    string `json:"url"`
	Code   string `json:"code"`
	Ca     string `json:"ca"`
}

type AccessTokenStatus struct {
	Redeemed bool   `json:"redeemed,omitempty"`
	Status   string `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AccessGrant struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          AccessGrantSpec   `json:"spec,omitempty"`
	Status        AccessGrantStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessGrantList contains a List of AccessGrant instances
type AccessGrantList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []AccessGrant `json:"items"`
}

type AccessGrantSpec struct {
	RedemptionsAllowed int    `json:"redemptionsAllowed,omitempty"`
	ExpirationWindow   string `json:"expirationWindow,omitempty"`
	Code               string `json:"code,omitempty"`
	Issuer             string `json:"issuer,omitempty"`
}

type AccessGrantStatus struct {
	Url        string `json:"url"`
	Code       string `json:"code"`
	Ca         string `json:"ca"`
	Redeemed  int    `json:"redeemed,omitempty"`
	Expiration string `json:"expiration,omitempty"`
	Status     string `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SecuredAccess struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          SecuredAccessSpec   `json:"spec,omitempty"`
	Status        SecuredAccessStatus `json:"status,omitempty"`
}

func (sa *SecuredAccess) Key() string {
	return fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecuredAccessList contains a List of SecuredAccess instances
type SecuredAccessList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []SecuredAccess `json:"items"`
}

type SecuredAccessPort struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

type SecuredAccessSpec struct {
	AccessType  string              `json:"accessType,omitempty"`
	Selector    map[string]string   `json:"selector"`
	Ports       []SecuredAccessPort `json:"ports"`
	Certificate string              `json:"certificate,omitempty"`
	Issuer      string              `json:"issuer,omitempty"`
	Options     map[string]string   `json:"options,omitempty"`
}

type SecuredAccessStatus struct {
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	Ca        string     `json:"ca,omitempty"`
	Status    string     `json:"status,omitempty"`
}

func (s *SecuredAccessStatus) GetEndpointByName(name string) *Endpoint {
	for i, endpoint := range s.Endpoints {
		if endpoint.Name == name {
			return &s.Endpoints[i]
		}
	}
	return nil
}

func (s *SecuredAccessStatus) UpdateEndpoint(endpoint *Endpoint) bool {
	current := s.GetEndpointByName(endpoint.Name)
	if current == nil {
		s.Endpoints = append(s.Endpoints, *endpoint)
		return true
	}
	if !current.MatchHostPort(endpoint) {
		current = endpoint
		return true
	}
	return false
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Certificate struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          CertificateSpec   `json:"spec,omitempty"`
	Status        CertificateStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CertificateList contains a List of Certificate instances
type CertificateList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Certificate `json:"items"`
}

type CertificateSpec struct {
	Ca      string   `json:"ca"`
	Subject string   `json:"subject"`
	Hosts   []string `json:"hosts,omitempty"`
	Client  bool     `json:"client,omitempty"`
	Server  bool     `json:"server,omitempty"`
	Signing bool     `json:"signing,omitempty"`
}

type CertificateStatus struct {
	Expiration string `json:"expiration,omitempty"`
	Status     string `json:"status,omitempty"`
}

func (c *Certificate) Key() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Name)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RouterAccess struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          RouterAccessSpec   `json:"spec,omitempty"`
	Status        RouterAccessStatus `json:"status,omitempty"`
}

func (r *RouterAccess) Key() string {
	return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
}

func (r *RouterAccess) FindRole(name string) *RouterAccessRole {
	for _, role := range r.Spec.Roles {
		if role.Name == name {
			return &role
		}
	}
	return nil
}

// endpoints are assumed all to be from one group
func (s *RouterAccessStatus) UpdateEndpointsForGroup(endpoints []Endpoint, group string) bool {
	all := []Endpoint{}
	updated := false
	byName := map[string]Endpoint{}
	for _, endpoint := range endpoints {
		endpoint.Group = group
		byName[endpoint.Name] = endpoint
	}
	for _, endpoint := range s.Endpoints {
		if endpoint.Group != group {
			all = append(all, endpoint)
		} else if desired, ok := byName[endpoint.Name]; ok {
			if desired.MatchHostPort(&endpoint) {
				all = append(all, endpoint)
			} else {
				all = append(all, desired)
				updated = true
			}
			delete(byName, endpoint.Name)
		} else {
			// endpoint is in group, but not in desired list so don't add it
			updated = true
		}
	}
	for _, endpoint := range byName {
		all = append(all, endpoint)
		updated = true
	}
	if updated {
		s.Endpoints = all
		return true
	}
	return false
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RouterAccessList contains a List of RouterAccess instances
type RouterAccessList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []RouterAccess `json:"items"`
}

type RouterAccessRole struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

func (role RouterAccessRole) GetPort() int32 {
	if role.Port != 0 {
		return int32(role.Port)
	} else if role.Name == "edge" {
		return 45671
	} else {
		return 55671
	}
}

type RouterAccessSpec struct {
	AccessType              string             `json:"accessType,omitempty"`
	Roles                   []RouterAccessRole `json:"roles"`
	TlsCredentials          string             `json:"tlsCredentials"`
	GenerateTlsCredentials  bool               `json:"generateTlsCredentials"`
	Issuer                  string             `json:"issuer"`
	Options                 map[string]string  `json:"options,omitempty"`
	BindHost                string             `json:"bindHost,omitempty"`
	SubjectAlternativeNames []string           `json:"subjectAlternativeNames,omitempty"`
}

type RouterAccessStatus struct {
	Status    `json:",inline"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AttachedConnector struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          AttachedConnectorSpec `json:"spec,omitempty"`
	Status        Status        `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AttachedConnectorList contains a List of AttachedConnector instances
type AttachedConnectorList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []AttachedConnector `json:"items"`
}

type AttachedConnectorSpec struct {
	SiteNamespace   string `json:"siteNamespace"`
	Selector        string `json:"selector,omitempty"`
	Port            int    `json:"port"`
	TlsCredentials  string `json:"tlsCredentials,omitempty"`
	Type            string `json:"type,omitempty"`
	IncludeNotReady bool   `json:"includeNotReady,omitempty"`
}


// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AttachedConnectorAnchor struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          AttachedConnectorAnchorSpec `json:"spec,omitempty"`
	Status        Status        `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AttachedConnectorAnchorList contains a List of AttachedConnectorAnchor instances
type AttachedConnectorAnchorList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []AttachedConnectorAnchor `json:"items"`
}

type AttachedConnectorAnchorSpec struct {
	ConnectorNamespace string `json:"connectorNamespace"`
	RoutingKey         string `json:"routingKey"`
}
