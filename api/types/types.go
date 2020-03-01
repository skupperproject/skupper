/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

const (
	// NamespaceDefault means the VAN is in the  skupper namespace which is applied when not specified by clients
	NamespaceDefault    string = "skupper"
	DefaultVanName      string = "skupper"
	ClusterLocalPostfix string = ".svc.cluster.local"
)

// KeyValue holds a key/value pair
type KeyValue struct {
	Key, Value string
}

// QdrMode describes how a qdr is intended to be deployed, either interior or edge
type QdrMode string

const (
	// QdrModeInterior means the qdr will participate in inter-router protocol exchanges
	QdrModeInterior QdrMode = "interior"
	// QdrModeEdge means that the qdr will connect to interior routers for network access
	QdrModeEdge = "edge"
)

// Qdr constants
const (
	QdrDeploymentName     string = "skupper-router"
	QdrComponentName      string = "router"
	DefaultQdrImage       string = "quay.io/interconnectedcloud/qdrouterd"
	QdrContainerName      string = "router"
	QdrLivenessPort       int32  = 9090
	QdrServiceAccountName string = "skupper"
	QdrViewRoleName       string = "skupper-view"
	QdrEnvConfig          string = "QDROUTERD_CONF"
)

var QdrViewPolicyRule = []rbacv1.PolicyRule{
	{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	},
}

var QdrPrometheusAnnotations = map[string]string{
	"prometheus.io/port":   "9090",
	"prometheus.io/scrape": "true",
}

// Controller constants
const (
	ControllerDeploymentName     string = "skupper-proxy-controller"
	ControllerComponentName      string = "controller"
	DefaultControllerImage       string = "quay.io/skupper/controller"
	ControllerContainerName      string = "proxy-controller"
	DefaultProxyImage            string = "quay.io/skupper/proxy"
	ControllerServiceAccountName string = "skupper-proxy-controller"
	ServiceSyncPath              string = "/etc/messaging/"
	ControllerEditRoleName       string = "skupper-edit"
)

var ControllerEditPolicyRule = []rbacv1.PolicyRule{
	{
		Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		APIGroups: []string{""},
		Resources: []string{"services", "configmaps"},
	},
	{
		Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		APIGroups: []string{"apps"},
		Resources: []string{"deployments", "statefulsets"},
	},
}

// OpenShift constants
const (
	OpenShiftServingCertSecretName string = "service.alpha.openshift.io/serving-cert-secret-name"
)

// Console constants
const (
	ConsolePortName                        string = "console"
	ConsoleServiceName                     string = "skupper-console"
	ConsoleDefaultServicePort              int32  = 8080
	ConsoleDefaultServiceTargetPort        int32  = 8080
	ConsoleOpenShiftServicePort            int32  = 8888
	ConsoleOpenShiftOauthServicePort       int32  = 443
	ConsoleOpenShiftOuathServiceTargetPort int32  = 8443
	ConsoleOpenShiftServingCerts           string = "skupper-proxy-certs"
)

type ConsoleAuthMode string

const (
	ConsoleAuthModeOpenshift ConsoleAuthMode = "openshift"
	ConsoleAuthModeInternal                  = "internal"
	ConsoleAuthModeUnsecured                 = "unsecured"
)

// Assembly constants
const (
	EdgeRole                string = "edge"
	EdgeRouteName           string = "skupper-edge"
	EdgeListenerPort        int32  = 45671
	InterRouterRole         string = "inter-router"
	InterRouterListenerPort int32  = 55671
	InterRouterRouteName    string = "skupper-inter-router"
	InterRouterProfile      string = "skupper-internal"

//    ConnectorSecretLabel    KeyValue = KeyValue{Key: "skupper.io/type", Value: "connection-token",}
)

// VanRouterSpec is the specification of VAN network with router, controller and assembly
type VanRouterSpec struct {
	Name           string          `json:"name,omitempty"`
	Namespace      string          `json:"namespace,omitempty"`
	AuthMode       ConsoleAuthMode `json:"authMode,omitempty"`
	Qdr            DeploymentSpec  `json:"router,omitempty"`
	Controller     DeploymentSpec  `json:"controller,omitempty"`
	Assembly       AssemblySpec    `json:"assembly,omitempty"`
	Users          []User          `json:"users,omitempty"`
	CertAuthoritys []CertAuthority `json:"certAuthoritys,omitempty"`
	Credentials    []Credential    `json:"credentials,omitempty"`
}

// DeploymentSpec for the VAN router or controller components to run within a cluster
type DeploymentSpec struct {
	Image           string                 `json:"image,omitempty"`
	Replicas        int32                  `json:"replicas,omitempty"`
	LivenessPort    int32                  `json:"livenessPort,omitempty"`
	Labels          map[string]string      `json:"labels,omitempty"`
	Annotations     map[string]string      `json:"annotations,omitempty"`
	EnvVar          []corev1.EnvVar        `json:"envVar,omitempty"`
	Ports           []corev1.ContainerPort `json:"ports,omitempty"`
	Volumes         []corev1.Volume        `json:"volumes,omitempty"`
	VolumeMounts    []corev1.VolumeMount   `json:"volumeMounts,omitempty"`
	Roles           []Role                 `json:"roles,omitempty"`
	RoleBindings    []RoleBinding          `json:"roleBinding,omitempty"`
	ServiceAccounts []ServiceAccount       `json:"serviceAccounts,omitempty"`
	Services        []Service              `json:"services,omitempty"`
}

// AssemblySpec for the links and connectors that form the VAN topology
type AssemblySpec struct {
	Name                  string       `json:"name,omitempty"`
	Mode                  string       `json:"mode,omitempty"`
	Listeners             []Listener   `json:"listeners,omitempty"`
	InterRouterListeners  []Listener   `json:"interRouterListeners,omitempty"`
	EdgeListeners         []Listener   `json:"edgeListeners,omitempty"`
	SslProfiles           []SslProfile `json:"sslProfiles,omitempty"`
	Connectors            []Connector  `json:"connectors,omitempty"`
	InterRouterConnectors []Connector  `json:"interRouterConnectors,omitempty"`
	EdgeConnectors        []Connector  `json:"edgeConnectors,omitempty"`
	Routes                []Route      `json:"routes,omitempty"`
}

type VanRouterStatusSpec struct {
	Mode             string            `json:"mode,omitempty"`
	QdrReadyReplicas int32             `json:"qdrReadyReplicas,omitempty"`
	ConnectedSites   QdrConnectedSites `json:"connectedSites,omitempty"`
	BindingsCount    int               `json:"bindingsCount,omitempty"`
}

type Listener struct {
	Name             string `json:"name,omitempty"`
	Host             string `json:"host,omitempty"`
	Port             int32  `json:"port"`
	RouteContainer   bool   `json:"routeContainer,omitempty"`
	Http             bool   `json:"http,omitempty"`
	Cost             int32  `json:"cost,omitempty"`
	SslProfile       string `json:"sslProfile,omitempty"`
	SaslMechanisms   string `json:"saslMechanisms,omitempty"`
	AuthenticatePeer bool   `json:"authenticatePeer,omitempty"`
	LinkCapacity     int32  `json:"linkCapacity,omitempty"`
}

type SslProfile struct {
	Name   string `json:"name,omitempty"`
	Cert   string `json:"cert,omitempty"`
	Key    string `json:"key,omitempty"`
	CaCert string `json:"caCert,omitempty"`
}

type ConnectorRole string

const (
	ConnectorRoleInterRouter ConnectorRole = "inter-router"
	ConnectorRoleEdge                      = "edge"
)

type Connector struct {
	Name           string `json:"name,omitempty"`
	Role           string `json:"role,omitempty"`
	Host           string `json:"host"`
	Port           string `json:"port"`
	RouteContainer bool   `json:"routeContainer,omitempty"`
	Cost           int32  `json:"cost,omitempty"`
	VerifyHostname bool   `json:"verifyHostname,omitempty"`
	SslProfile     string `json:"sslProfile,omitempty"`
	LinkCapacity   int32  `json:"linkCapacity,omitempty"`
}

type Role struct {
	Name  string              `json:"name,omitempty"`
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`
}

type RoleBinding struct {
	ServiceAccount string `json:"serviceAccount,omitempty"`
	Role           string `json:"role,omitempty"`
}

type ServiceAccount struct {
	ServiceAccount string            `json:"serviceAccount,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
}

type Route struct {
	Name          string
	TargetService string
	TargetPort    string
	Termination   routev1.TLSTerminationType
}

type Service struct {
	Name        string
	Type        string
	Ports       []corev1.ServicePort
	Annotations map[string]string
	Termination routev1.TLSTerminationType
}

type Credential struct {
	CA          string
	Name        string
	Subject     string
	Hosts       string
	ConnectJson bool
	Post        bool
}

type CertAuthority struct {
	Name string
}

type User struct {
	Name     string
	Password string
}

// TODO: rename this
type QdrConnectedSites struct {
	Direct   int
	Indirect int
	Total    int
}
