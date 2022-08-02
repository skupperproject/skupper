package v1alpha1

import (
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
	Spec          SiteSpec `json:"spec,omitempty"`
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
	ServiceAccount string `json:"serviceAccount,omitempty"`
	Settings       map[string]string `json:"settings,omitempty"`
}

type Status struct {
	Active        bool `json:"active"`
	StatusMessage string  `json:"status,omitempty"`
}

type SiteStatus struct {
	Status                         `json:",inline"`
	Endpoints         []Endpoint   `json:"endpoints,omitempty"`
	SitesInNetwork    int          `json:"sitesInNetwork,omitempty"`
	ServicesInNetwork int          `json:"servicesInNetwork,omitempty"`
	Network           []SiteRecord `json:"network,omitempty"`
}

type Endpoint struct {
	Name  string `json:"name,omitempty"`
	Host  string `json:"host,omitempty"`
	Port  string `json:"port,omitempty"`
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
	RoutingKey  string   `json:"routingKey"`
	Connectors  []string `json:"connectors"`
	Listeners   []string `json:"listeners"`
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
	Name    string `json:"name"`
	Port    int    `json:"port"`
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
	RoutingKey      string  `json:"routingKey"`
	Host            string  `json:"host,omitempty"`
	Selector        string  `json:"selector,omitempty"`
	Port            int     `json:"port"`
	TlsCredentials  string  `json:"tlsCredentials,omitempty"`
	Type            string  `json:"type,omitempty"`
	IncludeNotReady bool    `json:"includeNotReady,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LinkConfig struct {
	v1.TypeMeta                    `json:",inline"`
	v1.ObjectMeta                  `json:"metadata,omitempty"`
	Spec          LinkConfigSpec   `json:"spec,omitempty"`
	Status        LinkConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinkConfigList contains a List of LinkConfig instances
type LinkConfigList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []LinkConfig `json:"items"`
}

type LinkConfigSpec struct {
	InterRouter    HostPort `json:"interRouter"`
	Edge           HostPort `json:"edge"`
	TlsCredentials string   `json:"tlsCredentials,omitempty"`
	Cost           int      `json:"cost,omitempty"`
	NoClientAuth   bool     `json:"noClientAuth,omitempty"`
}

type HostPort struct {
	Host string           `json:"host"`
	Port int              `json:"port"`
}

type LinkConfigStatus struct {
	Status             `json:",inline"`
	Configured  bool   `json:"configured,omitempty"`
	Url         string `json:"url,omitempty"`
	Site        string `json:"site,omitempty"`
}
