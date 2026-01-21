package v2alpha1

import (
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatusType string

const (
	StatusReady   StatusType = "Ready"
	StatusPending StatusType = "Pending"
	StatusError   StatusType = "Error"
)

type ConditionState struct {
	Status  v1.ConditionStatus
	Reason  StatusType
	Message string
}

func ReadyCondition() ConditionState {
	return ConditionState{
		Status:  v1.ConditionTrue,
		Reason:  StatusReady,
		Message: STATUS_OK,
	}
}

func ErrorCondition(err error) ConditionState {
	return ConditionState{
		Status:  v1.ConditionFalse,
		Reason:  StatusError,
		Message: err.Error(),
	}
}

func PendingCondition(message string) ConditionState {
	return ConditionState{
		Status:  v1.ConditionFalse,
		Reason:  StatusPending,
		Message: message,
	}
}

func ErrorOrReadyCondition(err error) ConditionState {
	if err != nil {
		return ErrorCondition(err)
	}
	return ReadyCondition()
}

func ReadyOrPendingCondition(ready bool) ConditionState {
	if ready {
		return ReadyCondition()
	}
	return PendingCondition("Pending")
}

type Status struct {
	Conditions []v1.Condition `json:"conditions,omitempty"`
	StatusType StatusType     `json:"status,omitempty"`
	Message    string         `json:"message,omitempty"`
}

func (s *Status) readyState(requiredConditions []string) ConditionState {
	for _, conditionType := range requiredConditions {
		existing := meta.FindStatusCondition(s.Conditions, conditionType)
		if existing == nil {
			return PendingCondition("Not " + conditionType)
		} else if existing.Status == v1.ConditionFalse {
			return ConditionState{
				Status:  v1.ConditionFalse,
				Reason:  StatusType(existing.Reason),
				Message: existing.Message,
			}
		}
	}
	return ReadyCondition()
}

func (s *Status) setReady(requiredConditions []string, generation int64) bool {
	state := s.readyState(requiredConditions)
	changed := false
	if s.StatusType != state.Reason {
		s.StatusType = state.Reason
		changed = true
	}
	if s.Message != state.Message {
		s.Message = state.Message
		changed = true
	}
	if s.SetCondition(CONDITION_TYPE_READY, state, generation) {
		changed = true
	}
	return changed
}

func (s *Status) SetStatusMessage(message string) bool {
	if s.Message != message {
		s.Message = message
		return true
	}
	return false
}

func (s *Status) SetCondition(conditionType string, state ConditionState, generation int64) bool {
	condition := v1.Condition{
		Type:               conditionType,
		ObservedGeneration: generation,
		Status:             state.Status,
		Reason:             string(state.Reason),
		Message:            state.Message,
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

func (s *Site) DefaultIssuer() string {
	if s.Spec.DefaultIssuer != "" {
		return s.Spec.DefaultIssuer
	}
	if s.Status.DefaultIssuer != "" {
		return s.Status.DefaultIssuer
	}
	return "skupper-site-ca"
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
	Edge           bool              `json:"edge,omitempty"`
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

func (s *Site) SetConfigured(err error) bool {
	if s.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), s.ObjectMeta.Generation) {
		s.Status.setReady(s.requiredConditions(), s.ObjectMeta.Generation)
		return true
	}
	return false
}

func (s *Site) SetEndpoints(endpoints []Endpoint) bool {
	changed := false
	if !reflect.DeepEqual(s.Status.Endpoints, endpoints) {
		s.Status.Endpoints = endpoints
		changed = true
	}
	if s.Status.SetCondition(CONDITION_TYPE_RESOLVED, ReadyOrPendingCondition(len(s.Status.Endpoints) > 0), s.ObjectMeta.Generation) {
		s.Status.setReady(s.requiredConditions(), s.ObjectMeta.Generation)
		changed = true
	}
	return changed
}

func (s *Site) SetRunning(state ConditionState) bool {
	if s.Status.SetCondition(CONDITION_TYPE_RUNNING, state, s.ObjectMeta.Generation) {
		s.Status.setReady(s.requiredConditions(), s.ObjectMeta.Generation)
		return true
	}
	return false
}

func (s *Site) resolutionRequired() bool {
	return s.Spec.LinkAccess != "" && s.Spec.LinkAccess != "none"
}

func (s *Site) requiredConditions() []string {
	if s.resolutionRequired() {
		return []string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_RUNNING, CONDITION_TYPE_RESOLVED}
	}
	return []string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_RUNNING}
}

func (s *Site) IsConfigured() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_CONFIGURED)
}

func (s *Site) IsReady() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_READY)
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
	Controller     *Controller  `json:"controller,omitempty"`
}

type Controller struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Version   string `json:"version,omitempty"`
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
	Id        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
	Platform  string          `json:"platform,omitempty"`
	Version   string          `json:"version,omitempty"`
	Links     []LinkRecord    `json:"links,omitempty"`
	Services  []ServiceRecord `json:"services,omitempty"`
}

type ServiceRecord struct {
	RoutingKey string   `json:"routingKey,omitempty"`
	Connectors []string `json:"connectors,omitempty"`
	Listeners  []string `json:"listeners,omitempty"`
}

type LinkRecord struct {
	Name           string `json:"name,omitempty"`
	RemoteSiteId   string `json:"remoteSiteId,omitempty"`
	RemoteSiteName string `json:"remoteSiteName,omitempty"`
	Operational    bool   `json:"operational,omitempty"`
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
	if l.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), l.ObjectMeta.Generation) {
		l.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_MATCHED}, l.ObjectMeta.Generation)
		return true
	}
	return false
}

func (l *Listener) matched() ConditionState {
	if l.Status.HasMatchingConnector {
		return ReadyCondition()
	} else {
		return PendingCondition("No matching connectors")
	}
}

func (l *Listener) setMatched() bool {
	if l.Status.SetCondition(CONDITION_TYPE_MATCHED, l.matched(), l.ObjectMeta.Generation) {
		l.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_MATCHED}, l.ObjectMeta.Generation)
		return true
	}
	return false
}

func (l *Listener) SetHasMatchingConnector(value bool) bool {
	changed := false
	if l.Status.HasMatchingConnector != value {
		l.Status.HasMatchingConnector = value
		changed = true
	}
	if l.setMatched() {
		changed = true
	}
	return changed
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
	RoutingKey       string            `json:"routingKey"`
	Host             string            `json:"host"`
	Port             int               `json:"port"`
	TlsCredentials   string            `json:"tlsCredentials,omitempty"`
	Type             string            `json:"type,omitempty"`
	Observer         string            `json:"observer,omitempty"`
	ExposePodsByName bool              `json:"exposePodsByName,omitempty"`
	Settings         map[string]string `json:"settings,omitempty"`
}

type ListenerStatus struct {
	Status               `json:",inline"`
	HasMatchingConnector bool `json:"hasMatchingConnector,omitempty"`
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
	if c.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), c.ObjectMeta.Generation) {
		c.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_MATCHED}, c.ObjectMeta.Generation)
		return true
	}
	return false
}

func (c *Connector) matched() ConditionState {
	if c.Status.HasMatchingListener {
		return ReadyCondition()
	} else {
		return PendingCondition("No matching listeners")
	}
}

func (c *Connector) setMatched() bool {
	if c.Status.SetCondition(CONDITION_TYPE_MATCHED, c.matched(), c.ObjectMeta.Generation) {
		c.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_MATCHED}, c.ObjectMeta.Generation)
		return true
	}
	return false
}

func (c *Connector) SetHasMatchingListener(value bool) bool {
	changed := false
	if c.Status.HasMatchingListener != value {
		c.Status.HasMatchingListener = value
		changed = true
	}
	if c.setMatched() {
		changed = true
	}
	return changed
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
	RoutingKey          string            `json:"routingKey"`
	Host                string            `json:"host,omitempty"`
	Selector            string            `json:"selector,omitempty"`
	Port                int               `json:"port"`
	TlsCredentials      string            `json:"tlsCredentials,omitempty"`
	UseClientCert       bool              `json:"useClientCert,omitempty"`
	VerifyHostname      bool              `json:"verifyHostname,omitempty"`
	Type                string            `json:"type,omitempty"`
	ExposePodsByName    bool              `json:"exposePodsByName,omitempty"`
	IncludeNotReadyPods bool              `json:"includeNotReadyPods,omitempty"`
	Settings            map[string]string `json:"settings,omitempty"`
}

type PodDetails struct {
	UID  string `json:"-"`
	Name string `json:"name,omitempty"`
	IP   string `json:"ip,omitempty"`
}

type ConnectorStatus struct {
	Status              `json:",inline"`
	SelectedPods        []PodDetails `json:"selectedPods,omitempty"`
	HasMatchingListener bool         `json:"hasMatchingListener,omitempty"`
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
	if l.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), l.ObjectMeta.Generation) {
		l.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_OPERATIONAL}, l.ObjectMeta.Generation)
		return true
	}
	return false
}

func operationalState(operational bool) ConditionState {
	if operational {
		return ReadyCondition()
	} else {
		return PendingCondition("Not operational")
	}
}

func (l *Link) SetOperational(operational bool, remoteSiteId string, remoteSiteName string) bool {
	changed := false
	if l.Status.RemoteSiteId != remoteSiteId {
		l.Status.RemoteSiteId = remoteSiteId
		changed = true
	}
	if l.Status.RemoteSiteName != remoteSiteName {
		l.Status.RemoteSiteName = remoteSiteName
		changed = true
	}
	if l.Status.SetCondition(CONDITION_TYPE_OPERATIONAL, operationalState(operational), l.ObjectMeta.Generation) {
		l.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_OPERATIONAL}, l.ObjectMeta.Generation)
		return true
	}
	return changed
}

func (l *Link) IsConfigured() bool {
	return meta.IsStatusConditionTrue(l.Status.Conditions, CONDITION_TYPE_CONFIGURED)
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
	Endpoints      []Endpoint        `json:"endpoints"`
	TlsCredentials string            `json:"tlsCredentials,omitempty"`
	Cost           int               `json:"cost,omitempty"`
	Settings       map[string]string `json:"settings,omitempty"`
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
	Spec          AccessTokenSpec   `json:"spec,omitempty"`
	Status        AccessTokenStatus `json:"status,omitempty"`
}

func (t *AccessToken) SetRedeemed(err error) bool {
	state := ErrorOrReadyCondition(err)
	if t.Status.SetCondition(CONDITION_TYPE_REDEEMED, ErrorOrReadyCondition(err), t.ObjectMeta.Generation) {
		t.Status.Redeemed = t.IsRedeemed()
		t.Status.StatusType = state.Reason
		t.Status.Message = state.Message
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
	Url      string            `json:"url"`
	Code     string            `json:"code"`
	Ca       string            `json:"ca"`
	LinkCost int               `json:"linkCost,omitempty"`
	Settings map[string]string `json:"settings,omitempty"`
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

func (g *AccessGrant) resolved() ConditionState {
	if g.Status.Ca != "" && g.Status.Url != "" {
		return ReadyCondition()
	} else {
		return PendingCondition("Pending")
	}
}

func (g *AccessGrant) SetResolved() bool {
	if g.Status.SetCondition(CONDITION_TYPE_RESOLVED, g.resolved(), g.ObjectMeta.Generation) {
		g.Status.setReady([]string{CONDITION_TYPE_PROCESSED, CONDITION_TYPE_RESOLVED}, g.ObjectMeta.Generation)
		return true
	}
	return false
}

func (g *AccessGrant) SetProcessed(err error) bool {
	if g.Status.SetCondition(CONDITION_TYPE_PROCESSED, ErrorOrReadyCondition(err), g.ObjectMeta.Generation) {
		g.Status.setReady([]string{CONDITION_TYPE_PROCESSED, CONDITION_TYPE_RESOLVED}, g.ObjectMeta.Generation)
		return true
	}
	return false
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
	RedemptionsAllowed int               `json:"redemptionsAllowed,omitempty"`
	ExpirationWindow   string            `json:"expirationWindow,omitempty"`
	Code               string            `json:"code,omitempty"`
	Issuer             string            `json:"issuer,omitempty"`
	Settings           map[string]string `json:"settings,omitempty"`
}

type AccessGrantStatus struct {
	Status         `json:",inline"`
	Url            string `json:"url,omitempty"`
	Code           string `json:"code,omitempty"`
	Ca             string `json:"ca,omitempty"`
	Redemptions    int    `json:"redemptions,omitempty"`
	ExpirationTime string `json:"expirationTime,omitempty"`
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

func (s *SecuredAccess) SetConfigured(err error) bool {
	if s.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), s.ObjectMeta.Generation) {
		s.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_RESOLVED}, s.ObjectMeta.Generation)
		return true
	}
	return false
}

func (s *SecuredAccess) SetResolved(endpoints []Endpoint) bool {
	changed := false
	if len(endpoints) != len(s.Status.Endpoints) || !reflect.DeepEqual(s.Status.Endpoints, endpoints) {
		s.Status.Endpoints = endpoints
		changed = true
	}
	if s.Status.SetCondition(CONDITION_TYPE_RESOLVED, ReadyOrPendingCondition(len(endpoints) > 0), s.ObjectMeta.Generation) {
		changed = true
	}
	if s.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_RESOLVED}, s.ObjectMeta.Generation) {
		changed = true
	}
	return changed
}

func (s *SecuredAccess) IsReady() bool {
	return meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_CONFIGURED) &&
		meta.IsStatusConditionTrue(s.Status.Conditions, CONDITION_TYPE_RESOLVED)
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
	Settings    map[string]string   `json:"settings,omitempty"`
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
		*current = *endpoint
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
	Ca       string            `json:"ca"`
	Subject  string            `json:"subject"`
	Hosts    []string          `json:"hosts,omitempty"`
	Client   bool              `json:"client,omitempty"`
	Server   bool              `json:"server,omitempty"`
	Signing  bool              `json:"signing,omitempty"`
	Settings map[string]string `json:"settings,omitempty"`
}

type CertificateStatus struct {
	Status     `json:",inline"`
	Expiration string `json:"expiration,omitempty"`
}

func (c *Certificate) Key() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Name)
}

func (c *Certificate) SetReady(err error) bool {
	return c.Status.SetCondition(CONDITION_TYPE_READY, ErrorOrReadyCondition(err), c.ObjectMeta.Generation)
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
	if r.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), r.ObjectMeta.Generation) {
		r.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_RESOLVED}, r.ObjectMeta.Generation)
		return true
	}
	return false
}

func (r *RouterAccess) Resolve(endpoints []Endpoint, group string) bool {
	changed := false
	if r.Status.UpdateEndpointsForGroup(endpoints, group) {
		changed = true
	}
	if r.Status.SetCondition(CONDITION_TYPE_RESOLVED, ReadyOrPendingCondition(len(r.Status.Endpoints) > 0), r.ObjectMeta.Generation) {
		r.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_RESOLVED}, r.ObjectMeta.Generation)
		changed = true
	}
	return changed
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
	Port int    `json:"port,omitempty"`
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
	GenerateTlsCredentials  bool               `json:"generateTlsCredentials,omitempty"`
	Issuer                  string             `json:"issuer,omitempty"`
	BindHost                string             `json:"bindHost,omitempty"`
	SubjectAlternativeNames []string           `json:"subjectAlternativeNames,omitempty"`
	Settings                map[string]string  `json:"settings,omitempty"`
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
	if c.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), c.ObjectMeta.Generation) {
		c.Status.setReady([]string{CONDITION_TYPE_CONFIGURED}, c.ObjectMeta.Generation)
		return true
	}
	return false
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
	SiteNamespace       string            `json:"siteNamespace"`
	Selector            string            `json:"selector"`
	Port                int               `json:"port"`
	TlsCredentials      string            `json:"tlsCredentials,omitempty"`
	UseClientCert       bool              `json:"useClientCert,omitempty"`
	Type                string            `json:"type,omitempty"`
	IncludeNotReadyPods bool              `json:"includeNotReadyPods,omitempty"`
	Settings            map[string]string `json:"settings,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AttachedConnectorBinding struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	Spec          AttachedConnectorBindingSpec   `json:"spec,omitempty"`
	Status        AttachedConnectorBindingStatus `json:"status,omitempty"`
}

type AttachedConnectorBindingStatus struct {
	Status              `json:",inline"`
	HasMatchingListener bool `json:"hasMatchingListener,omitempty"`
}

func (c *AttachedConnectorBinding) SetConfigured(err error) bool {
	if c.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), c.ObjectMeta.Generation) {
		c.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_MATCHED}, c.ObjectMeta.Generation)
		return true
	}
	return false
}

func (c *AttachedConnectorBinding) setMatched() bool {
	if c.Status.SetCondition(CONDITION_TYPE_MATCHED, ReadyOrPendingCondition(c.Status.HasMatchingListener), c.ObjectMeta.Generation) {
		c.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_MATCHED}, c.ObjectMeta.Generation)
		return true
	}
	return false
}

func (c *AttachedConnectorBinding) SetHasMatchingListener(value bool) bool {
	if c.Status.HasMatchingListener != value {
		c.Status.HasMatchingListener = value
		c.setMatched()
		return true
	}
	return false
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AttachedConnectorBindingList contains a List of AttachedConnectorBinding instances
type AttachedConnectorBindingList struct {
	v1.TypeMeta `json:",inline"`
	v1.ListMeta `json:"metadata,omitempty"`
	Items       []AttachedConnectorBinding `json:"items"`
}

type AttachedConnectorBindingSpec struct {
	ConnectorNamespace string            `json:"connectorNamespace"`
	RoutingKey         string            `json:"routingKey"`
	ExposePodsByName   bool              `json:"exposePodsByName,omitempty"`
	Settings           map[string]string `json:"settings,omitempty"`
}
