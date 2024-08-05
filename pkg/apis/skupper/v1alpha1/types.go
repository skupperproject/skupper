package v1alpha1

import (
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
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
	Conditions    []v1.Condition `json:"conditions,omitempty"`
	StatusMessage string         `json:"status,omitempty"`
}

func (s *Status) SetStatusMessage(message string) bool {
	if s.StatusMessage != message {
		s.StatusMessage = message
		return true
	}
	return false
}

func (s *Status) SetCondition(conditionType string, err error, generation int64) bool {
	condition := v1.Condition{
		Type:               conditionType,
		ObservedGeneration: generation,
	}
	if err != nil {
		condition.Status = v1.ConditionFalse
		condition.Reason = "Error"
		condition.Message = err.Error()
	} else {
		condition.Status = v1.ConditionTrue
		condition.Reason = STATUS_OK
		condition.Message = STATUS_OK
	}
	return setStatusCondition(&s.Conditions, condition)
}

func setStatusCondition(conditions *[]v1.Condition, newCondition v1.Condition) (changed bool) {
	if conditions == nil {
		return false
	}
	existingCondition := meta.FindStatusCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = v1.NewTime(time.Now())
		}
		*conditions = append(*conditions, newCondition)
		return true
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = v1.NewTime(time.Now())
		}
		changed = true
	}

	if existingCondition.Reason != newCondition.Reason {
		existingCondition.Reason = newCondition.Reason
		changed = true
	}
	if existingCondition.Message != newCondition.Message {
		existingCondition.Message = newCondition.Message
		changed = true
	}
	if existingCondition.ObservedGeneration != newCondition.ObservedGeneration {
		existingCondition.ObservedGeneration = newCondition.ObservedGeneration
		changed = true
	}

	return changed
}

func (s *Site) SetConfigured(err error) bool {
	if s.Status.SetCondition(CONDITION_TYPE_CONFIGURED, err, s.ObjectMeta.Generation) {
		s.setReady(err)
		return true
	}
	return false
}

func (s *Site) SetResolved(err error) bool {
	if s.Status.SetCondition(CONDITION_TYPE_RESOLVED, err, s.ObjectMeta.Generation) {
		s.setReady(err)
		return true
	}
	return false
}

func (s *Site) SetRunning(err error) bool {
	if s.Status.SetCondition(CONDITION_TYPE_RUNNING, err, s.ObjectMeta.Generation) {
		s.setReady(err)
		return true
	}
	return false
}

func (s *Site) setReady(err error) bool {
	if err != nil {
		s.Status.StatusMessage = err.Error()
		return s.Status.SetCondition(CONDITION_TYPE_READY, err, s.ObjectMeta.Generation)
	} else if s.IsReady() {
		s.Status.StatusMessage = STATUS_OK
		return s.Status.SetCondition(CONDITION_TYPE_READY, nil, s.ObjectMeta.Generation)
	}
	return false
}

func (s *Site) IsReady() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_CONFIGURED) &&
		(!s.resolutionRequired() || meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_RESOLVED)) &&
		meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_RUNNING)
}

func (s *Site) IsConfigured() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_CONFIGURED)
}

func (s *Site) resolutionRequired() bool {
	return s.Spec.LinkAccess != "" && s.Spec.LinkAccess != "none"
}

const STATUS_OK = "OK"

const CONDITION_TYPE_CONFIGURED = "Configured"
const CONDITION_TYPE_RESOLVED = "Resolved"
const CONDITION_TYPE_RUNNING = "Running"
const CONDITION_TYPE_MATCHED = "Matched"
const CONDITION_TYPE_PROCESSED = "Processed"
const CONDITION_TYPE_REDEEMED = "Redeemed"
const CONDITION_TYPE_OPERATIONAL = "Operational"
const CONDITION_TYPE_READY = "Ready"

type SiteStatus struct {
	Status         `json:",inline"`
	Endpoints      []Endpoint   `json:"endpoints,omitempty"`
	SitesInNetwork int          `json:"sitesInNetwork,omitempty"`
	Network        []SiteRecord `json:"network,omitempty"`
	DefaultIssuer  string       `json:"defaultIssuer,omitempty"`
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
	Links     []LinkRecord    `json:"links,omitempty"`
	Services  []ServiceRecord `json:"services,omitempty"`
}

type ServiceRecord struct {
	RoutingKey string   `json:"routingKey"`
	Connectors []string `json:"connectors"`
	Listeners  []string `json:"listeners"`
}

type LinkRecord struct {
	Name           string `json:"name"`
	RemoteSiteId   string `json:"remoteSiteId"`
	RemoteSiteName string `json:"remoteSiteName"`
	Operational    bool   `json:"operational"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Listener struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          ListenerSpec   `json:"spec,omitempty"`
	Status        ListenerStatus `json:"status,omitempty"`
}

func (l *Listener) SetConfigured(err error) bool {
	if l.Status.SetCondition(CONDITION_TYPE_CONFIGURED, err, l.ObjectMeta.Generation) {
		l.setReady(err)
		return true
	}
	return false
}

func (c *Listener) setMatched() bool {
	var err error
	if c.Status.MatchingConnectorCount == 0 {
		err = fmt.Errorf("No matching connectors")
	}
	if c.Status.SetCondition(CONDITION_TYPE_MATCHED, err, c.ObjectMeta.Generation) {
		c.setReady(err)
		return true
	}
	return false
}

func (c *Listener) SetMatchingConnectorCount(count int) bool {
	if c.Status.MatchingConnectorCount != count {
		c.Status.MatchingConnectorCount = count
		c.setMatched()
		return true
	}
	return false
}

func (l *Listener) setReady(err error) bool {
	if err != nil {
		l.Status.StatusMessage = err.Error()
		return l.Status.SetCondition(CONDITION_TYPE_READY, err, l.ObjectMeta.Generation)
	} else if l.isReady() {
		l.Status.StatusMessage = STATUS_OK
		return l.Status.SetCondition(CONDITION_TYPE_READY, nil, l.ObjectMeta.Generation)
	}
	return false
}

func (l *Listener) isReady() bool {
	return meta.IsStatusConditionTrue(l.Status.Conditions, CONDITION_TYPE_CONFIGURED) &&
		meta.IsStatusConditionTrue(l.Status.Conditions, CONDITION_TYPE_MATCHED)
}

func (l *Listener) Protocol() corev1.Protocol {
	if l.Spec.Type == "udp" {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}

func (s *Listener) IsConfigured() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_CONFIGURED)
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

type ListenerStatus struct {
	Status                 `json:",inline"`
	MatchingConnectorCount int `json:"matchingConnectorCount,omitempty"`
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
	Spec          ConnectorSpec   `json:"spec,omitempty"`
	Status        ConnectorStatus `json:"status,omitempty"`
}

func (c *Connector) SetConfigured(err error) bool {
	if c.Status.SetCondition(CONDITION_TYPE_CONFIGURED, err, c.ObjectMeta.Generation) {
		c.setReady(err)
		return true
	}
	return false
}

func (c *Connector) setMatched() bool {
	var err error
	if c.Status.MatchingListenerCount == 0 {
		err = fmt.Errorf("No matching listeners")
	}
	if c.Status.SetCondition(CONDITION_TYPE_MATCHED, err, c.ObjectMeta.Generation) {
		c.setReady(err)
		return true
	}
	return false
}

func (c *Connector) SetMatchingListenerCount(count int) bool {
	if c.Status.MatchingListenerCount != count {
		c.Status.MatchingListenerCount = count
		c.setMatched()
		return true
	}
	return false
}

func (c *Connector) setReady(err error) bool {
	if err != nil {
		c.Status.StatusMessage = err.Error()
		return c.Status.SetCondition(CONDITION_TYPE_READY, err, c.ObjectMeta.Generation)
	} else if c.isReady() {
		c.Status.StatusMessage = STATUS_OK
		return c.Status.SetCondition(CONDITION_TYPE_READY, nil, c.ObjectMeta.Generation)
	}
	return false
}

func (c *Connector) isReady() bool {
	return meta.IsStatusConditionTrue(c.Status.Conditions, CONDITION_TYPE_CONFIGURED) &&
		meta.IsStatusConditionTrue(c.Status.Conditions, CONDITION_TYPE_MATCHED)
}

func (c *Connector) SetSelectedPods(pods []PodDetails) bool {
	if !reflect.DeepEqual(pods, c.Status.SelectedPods) {
		c.Status.SelectedPods = pods
		return true
	}
	return false
}

func (s *Connector) IsConfigured() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_CONFIGURED)
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

type PodDetails struct {
	UID  string `json:"-"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type ConnectorStatus struct {
	Status                `json:",inline"`
	SelectedPods          []PodDetails `json:"selectedPods,omitempty"`
	MatchingListenerCount int          `json:"matchingListenerCount,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Link struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          LinkSpec   `json:"spec,omitempty"`
	Status        LinkStatus `json:"status,omitempty"`
}

func (l *Link) SetConfigured(err error) bool {
	if l.Status.SetCondition(CONDITION_TYPE_CONFIGURED, err, l.ObjectMeta.Generation) {
		l.setReady(err)
		return true
	}
	return false
}

func (l *Link) SetOperational(operational bool, remoteSiteId string, remoteSiteName string) bool {
	var err error
	if !operational {
		err = fmt.Errorf("Not operational")
	}
	changed := false
	if l.Status.RemoteSiteId != remoteSiteId {
		l.Status.RemoteSiteId = remoteSiteId
		changed = true
	}
	if l.Status.RemoteSiteName != remoteSiteName {
		l.Status.RemoteSiteName = remoteSiteName
		changed = true
	}
	if l.Status.SetCondition(CONDITION_TYPE_OPERATIONAL, err, l.ObjectMeta.Generation) {
		l.setReady(err)
		return true
	}
	return changed
}

func (l *Link) setReady(err error) bool {
	if err != nil {
		l.Status.StatusMessage = err.Error()
		return l.Status.SetCondition(CONDITION_TYPE_READY, err, l.ObjectMeta.Generation)
	} else if l.IsReady() {
		l.Status.StatusMessage = STATUS_OK
		return l.Status.SetCondition(CONDITION_TYPE_READY, nil, l.ObjectMeta.Generation)
	}
	return false
}

func (l *Link) IsReady() bool {
	return meta.IsStatusConditionTrue(l.Status.Conditions, CONDITION_TYPE_CONFIGURED) &&
		meta.IsStatusConditionTrue(l.Status.Conditions, CONDITION_TYPE_OPERATIONAL)
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
	Status         `json:",inline"`
	RemoteSiteId   string `json:"remoteSiteId,omitempty"`
	RemoteSiteName string `json:"remoteSiteName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AccessToken struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          AccessTokenSpec `json:"spec,omitempty"`
	Status        Status          `json:"status,omitempty"`
}

func (t *AccessToken) SetRedeemed(err error) bool {
	if t.Status.SetCondition(CONDITION_TYPE_REDEEMED, err, t.ObjectMeta.Generation) {
		if err == nil {
			t.Status.StatusMessage = STATUS_OK
		} else if err != nil {
			t.Status.StatusMessage = err.Error()
		}
		return true
	}
	return false
}

func (t *AccessToken) IsRedeemed() bool {
	return meta.IsStatusConditionTrue(t.Status.Conditions, CONDITION_TYPE_REDEEMED)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessTokenList contains a List of AccessToken instances
type AccessTokenList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []AccessToken `json:"items"`
}

type AccessTokenSpec struct {
	Url  string `json:"url"`
	Code string `json:"code"`
	Ca   string `json:"ca"`
}

type AccessTokenStatus struct {
	Status   `json:",inline"`
	Redeemed bool `json:"redeemed,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AccessGrant struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          AccessGrantSpec   `json:"spec,omitempty"`
	Status        AccessGrantStatus `json:"status,omitempty"`
}

func (g *AccessGrant) SetResolved() bool {
	var err error
	if g.Status.Ca == "" || g.Status.Url == "" {
		err = fmt.Errorf("Pending")
	}
	if g.Status.SetCondition(CONDITION_TYPE_RESOLVED, err, g.ObjectMeta.Generation) {
		g.setReady(err)
		return true
	}
	return false
}

func (g *AccessGrant) SetProcessed(err error) bool {
	if g.Status.SetCondition(CONDITION_TYPE_PROCESSED, err, g.ObjectMeta.Generation) {
		g.setReady(err)
		return true
	}
	return false
}

func (g *AccessGrant) setReady(err error) bool {
	if err != nil {
		g.Status.StatusMessage = err.Error()
		return g.Status.SetCondition(CONDITION_TYPE_READY, err, g.ObjectMeta.Generation)
	} else if g.isReady() {
		g.Status.StatusMessage = STATUS_OK
		return g.Status.SetCondition(CONDITION_TYPE_READY, nil, g.ObjectMeta.Generation)
	}
	return false
}

func (g *AccessGrant) isReady() bool {
	return meta.IsStatusConditionTrue(g.Status.Conditions, CONDITION_TYPE_PROCESSED) &&
		meta.IsStatusConditionTrue(g.Status.Conditions, CONDITION_TYPE_RESOLVED)
}

func (s *AccessGrant) IsReady() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_READY)
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
	Status     `json:",inline"`
	Url        string `json:"url"`
	Code       string `json:"code"`
	Ca         string `json:"ca"`
	Redeemed   int    `json:"redeemed,omitempty"`
	Expiration string `json:"expiration,omitempty"`
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
	Status    `json:",inline"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	Ca        string     `json:"ca,omitempty"`
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
	Status     `json:",inline"`
	Expiration string `json:"expiration,omitempty"`
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

func (r *RouterAccess) SetConfigured(err error) bool {
	if r.Status.SetCondition(CONDITION_TYPE_CONFIGURED, err, r.ObjectMeta.Generation) {
		r.setReady(err)
		return true
	}
	return false
}

func (r *RouterAccess) Resolve(endpoints []Endpoint, group string) bool {
	changed := false
	if r.Status.UpdateEndpointsForGroup(endpoints, group) {
		changed = true
	}
	if len(r.Status.Endpoints) > 0 && r.Status.SetCondition(CONDITION_TYPE_RESOLVED, nil, r.ObjectMeta.Generation) {
		r.setReady(nil)
		changed = true
	}
	return changed
}

func (r *RouterAccess) setReady(err error) bool {
	if err != nil {
		r.Status.StatusMessage = err.Error()
		return r.Status.SetCondition(CONDITION_TYPE_READY, err, r.ObjectMeta.Generation)
	} else if r.isReady() {
		r.Status.StatusMessage = STATUS_OK
		return r.Status.SetCondition(CONDITION_TYPE_READY, nil, r.ObjectMeta.Generation)
	}
	return false
}

func (r *RouterAccess) isReady() bool {
	return meta.IsStatusConditionTrue(r.Status.Conditions, CONDITION_TYPE_CONFIGURED) &&
		meta.IsStatusConditionTrue(r.Status.Conditions, CONDITION_TYPE_RESOLVED)
}

func (r *RouterAccess) IsConfigured() bool {
	return meta.IsStatusConditionTrue(r.Status.Conditions, CONDITION_TYPE_CONFIGURED)
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
	Spec          AttachedConnectorSpec   `json:"spec,omitempty"`
	Status        AttachedConnectorStatus `json:"status,omitempty"`
}

type AttachedConnectorStatus struct {
	Status       `json:",inline"`
	SelectedPods []PodDetails `json:"selectedPods,omitempty"`
}

func (c *AttachedConnector) SetConfigured(err error) bool {
	if c.Status.SetCondition(CONDITION_TYPE_CONFIGURED, err, c.ObjectMeta.Generation) {
		c.setReady(err)
		return true
	}
	return false
}

func (c *AttachedConnector) setReady(err error) bool {
	if err != nil {
		c.Status.StatusMessage = err.Error()
		return c.Status.SetCondition(CONDITION_TYPE_READY, err, c.ObjectMeta.Generation)
	} else if c.isReady() {
		c.Status.StatusMessage = STATUS_OK
		return c.Status.SetCondition(CONDITION_TYPE_READY, nil, c.ObjectMeta.Generation)
	}
	return false
}

func (c *AttachedConnector) isReady() bool {
	return meta.IsStatusConditionTrue(c.Status.Conditions, CONDITION_TYPE_CONFIGURED)
}

func (c *AttachedConnector) SetSelectedPods(pods []PodDetails) bool {
	if !reflect.DeepEqual(pods, c.Status.SelectedPods) {
		c.Status.SelectedPods = pods
		return true
	}
	return false
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
	Spec          AttachedConnectorAnchorSpec   `json:"spec,omitempty"`
	Status        AttachedConnectorAnchorStatus `json:"status,omitempty"`
}

type AttachedConnectorAnchorStatus struct {
	Status                `json:",inline"`
	MatchingListenerCount int `json:"matchingListenerCount,omitempty"`
}

func (c *AttachedConnectorAnchor) SetConfigured(err error) bool {
	if c.Status.SetCondition(CONDITION_TYPE_CONFIGURED, err, c.ObjectMeta.Generation) {
		c.setReady(err)
		return true
	}
	return false
}

func (c *AttachedConnectorAnchor) setReady(err error) bool {
	if err != nil {
		c.Status.StatusMessage = err.Error()
		return c.Status.SetCondition(CONDITION_TYPE_READY, err, c.ObjectMeta.Generation)
	} else if c.isReady() {
		c.Status.StatusMessage = STATUS_OK
		return c.Status.SetCondition(CONDITION_TYPE_READY, nil, c.ObjectMeta.Generation)
	}
	return false
}

func (c *AttachedConnectorAnchor) setMatched() bool {
	var err error
	if c.Status.MatchingListenerCount == 0 {
		err = fmt.Errorf("No matching listeners")
	}
	if c.Status.SetCondition(CONDITION_TYPE_MATCHED, err, c.ObjectMeta.Generation) {
		c.setReady(err)
		return true
	}
	return false
}

func (c *AttachedConnectorAnchor) SetMatchingListenerCount(count int) bool {
	if c.Status.MatchingListenerCount != count || c.Status.MatchingListenerCount == 0 {
		c.Status.MatchingListenerCount = count
		c.setMatched()
		return true
	}
	return false
}

func (c *AttachedConnectorAnchor) isReady() bool {
	return meta.IsStatusConditionTrue(c.Status.Conditions, CONDITION_TYPE_CONFIGURED) &&
		meta.IsStatusConditionTrue(c.Status.Conditions, CONDITION_TYPE_MATCHED)
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
