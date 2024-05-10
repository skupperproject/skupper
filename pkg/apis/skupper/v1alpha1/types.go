package v1alpha1

import (
	"fmt"

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiteList contains a List of Site instances
type SiteList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Site `json:"items"`
}

type SiteSpec struct {
	ServiceAccount string            `json:"serviceAccount,omitempty"`
	Settings       map[string]string `json:"settings,omitempty"`
}

type Status struct {
	Active        bool   `json:"active"`
	StatusMessage string `json:"status,omitempty"`
}

type SiteStatus struct {
	Status            `json:",inline"`
	Endpoints         []Endpoint   `json:"endpoints,omitempty"`
	SitesInNetwork    int          `json:"sitesInNetwork,omitempty"`
	ServicesInNetwork int          `json:"servicesInNetwork,omitempty"`
	Network           []SiteRecord `json:"network,omitempty"`
}

type Endpoint struct {
	Name string `json:"name,omitempty"`
	Host string `json:"host,omitempty"`
	Port string `json:"port,omitempty"`
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
	InterRouter    HostPort `json:"interRouter"`
	Edge           HostPort `json:"edge"`
	TlsCredentials string   `json:"tlsCredentials,omitempty"`
	Cost           int      `json:"cost,omitempty"`
	NoClientAuth   bool     `json:"noClientAuth,omitempty"`
}

type HostPort struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type LinkStatus struct {
	Status     `json:",inline"`
	Configured bool   `json:"configured,omitempty"`
	Url        string `json:"url,omitempty"`
	Site       string `json:"site,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Claim struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          ClaimSpec   `json:"spec,omitempty"`
	Status        ClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClaimList contains a List of Claim instances
type ClaimList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Claim `json:"items"`
}

type ClaimSpec struct {
	Url    string `json:"url"`
	Secret string `json:"secret"`
	Ca     string `json:"ca"`
}

type ClaimStatus struct {
	Claimed bool   `json:"claimed,omitempty"`
	Status  string `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Grant struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          GrantSpec   `json:"spec,omitempty"`
	Status        GrantStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GrantList contains a List of Grant instances
type GrantList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []Grant `json:"items"`
}

type GrantSpec struct {
	Claims   int    `json:"claims,omitempty"`
	ValidFor string `json:"validFor,omitempty"`
	Secret   string `json:"secret,omitempty"`
}

type GrantStatus struct {
	Url        string `json:"url"`
	Secret     string `json:"secret"`
	Ca         string `json:"ca"`
	Claimed    int    `json:"claimed,omitempty"`
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
	Ca          string              `json:"ca,omitempty"`
	Options     map[string]string   `json:"options,omitempty"`
}

type SecuredAccessUrl struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

func (s *SecuredAccessUrl) AsLinkAccessUrl() LinkAccessUrl {
	return LinkAccessUrl{
		Role: s.Name,
		Url:  s.Url,
	}
}

type SecuredAccessStatus struct {
	Urls   []SecuredAccessUrl `json:"urls,omitempty"`
	Ca     string             `json:"ca,omitempty"`
	Status string             `json:"status,omitempty"`
}

func (s *SecuredAccessStatus) GetLinkAccessUrls() []LinkAccessUrl {
	var urls []LinkAccessUrl
	for _, u := range s.Urls {
		urls = append(urls, u.AsLinkAccessUrl())
	}
	return urls
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

type LinkAccess struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          LinkAccessSpec   `json:"spec,omitempty"`
	Status        LinkAccessStatus `json:"status,omitempty"`
}

func (link *LinkAccess) Key() string {
	return fmt.Sprintf("%s/%s", link.Namespace, link.Name)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinkAccessList contains a List of LinkAccess instances
type LinkAccessList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []LinkAccess `json:"items"`
}

type LinkAccessRole struct {
	Role string `json:"role"`
	Port int    `json:"port"`
}

type LinkAccessSpec struct {
	AccessType              string            `json:"accessType,omitempty"`
	Roles                   []LinkAccessRole  `json:"roles"`
	TlsCredentials          string            `json:"tlsCredentials"`
	Ca                      string            `json:"ca"`
	Options                 map[string]string `json:"options,omitempty"`
	BindHosts               []string          `json:"bindHosts,omitempty"`
	SubjectAlternativeNames []string          `json:"subjectAlternativeNames,omitempty"`
}

type LinkAccessUrl struct {
	Role string `json:"role"`
	Url  string `json:"url"`
}

type LinkAccessStatus struct {
	Active bool            `json:"active,omitempty"`
	Status string          `json:"status,omitempty"`
	Urls   []LinkAccessUrl `json:"urls,omitempty"`
}
