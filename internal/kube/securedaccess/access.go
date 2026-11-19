package securedaccess

import (
	"context"
	"log/slog"
	"maps"
	"reflect"
	"sort"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/resource"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ControllerContext interface {
	IsControlled(namespace string) bool
	SnapshotMetadata(namespace string) *labels.LabelsAndAnnotations
	SetLabels(namespace, name, kind string, labels map[string]string) bool
	SetAnnotations(namespace, name, kind string, annotations map[string]string) bool
	Namespace() string
}

type SecuredAccessManager struct {
	clients           internalclient.Clients
	context           ControllerContext
	config            Config
	defaultAccessType string
	watcher           *ResourceCaches
	gateway           *gatewayScope
	certificates      *accessCertificateScope
	logger            *slog.Logger
	applyCache        *resource.SSASuccessCache
}

func NewSecuredAccessManager(clients internalclient.Clients, config *Config, ctxt ControllerContext) *SecuredAccessManager {
	c := *config
	c.EnabledAccessTypes = append([]string(nil), config.EnabledAccessTypes...)
	m := &SecuredAccessManager{clients: clients, context: ctxt, config: c, defaultAccessType: c.getDefaultAccessType(clients), logger: slog.Default().With("component", "kube.securedaccess.manager")}
	m.applyCache = resource.NewSSASuccessCache(1024, applyFieldManager)
	m.gateway = &gatewayScope{manager: m}
	m.certificates = &accessCertificateScope{manager: m}
	return m
}

func (m *SecuredAccessManager) IsValidAccessType(accessType string) bool {
	return m.config.isValidAccessType(accessType)
}
func (m *SecuredAccessManager) actualAccessType(sa *skupperv2alpha1.SecuredAccess) string {
	if sa.Spec.AccessType == "" {
		return m.defaultAccessType
	}
	return sa.Spec.AccessType
}
func (m *SecuredAccessManager) isControlled(namespace string) bool {
	return m.context == nil || m.context.IsControlled(namespace)
}

// Ensure deprecated - makes unconditional api requests
func (m *SecuredAccessManager) Ensure(namespace, name string, spec skupperv2alpha1.SecuredAccessSpec, annotations map[string]string, refs []metav1.OwnerReference) error {
	client := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(namespace)
	current, err := client.Get(context.Background(), name, metav1.GetOptions{})
	create := apierrors.IsNotFound(err)
	if err != nil && !create {
		return err
	}
	if create {
		current = &skupperv2alpha1.SecuredAccess{TypeMeta: metav1.TypeMeta{APIVersion: "skupper.io/v2alpha1", Kind: "SecuredAccess"}, ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	}
	desired := current.DeepCopy()
	desired.Spec = spec
	desired.OwnerReferences = append([]metav1.OwnerReference(nil), refs...)
	if desired.Labels == nil {
		desired.Labels = map[string]string{}
	}
	if desired.Annotations == nil {
		desired.Annotations = map[string]string{}
	}
	for k, v := range annotations {
		desired.Annotations[k] = v
	}
	if m.context != nil {
		m.context.SetLabels(namespace, name, "SecuredAccess", desired.Labels)
		m.context.SetAnnotations(namespace, name, "SecuredAccess", desired.Annotations)
	}
	if create {
		_, err = client.Create(context.Background(), desired, metav1.CreateOptions{})
	} else if !reflect.DeepEqual(current.Spec, desired.Spec) || !reflect.DeepEqual(current.OwnerReferences, desired.OwnerReferences) || !maps.Equal(current.Labels, desired.Labels) || !maps.Equal(current.Annotations, desired.Annotations) {
		_, err = client.Update(context.Background(), desired, metav1.UpdateOptions{})
	}
	return err
}

// Delete deprecated
func (m *SecuredAccessManager) Delete(namespace, name string) error {
	err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func ownerReferences(sa *skupperv2alpha1.SecuredAccess) []metav1.OwnerReference {
	return []metav1.OwnerReference{{Kind: "SecuredAccess", APIVersion: "skupper.io/v2alpha1", Name: sa.Name, UID: sa.UID}}
}

func getHosts(sa *skupperv2alpha1.SecuredAccess) []string {
	hosts := map[string]bool{sa.Name: true, sa.Name + "." + sa.Namespace: true}
	for _, e := range sa.Status.Endpoints {
		if e.Host != "" {
			hosts[e.Host] = true
		}
	}
	result := make([]string, 0, len(hosts))
	for h := range hosts {
		result = append(result, h)
	}
	sort.Strings(result)
	return result
}

func serviceType(accessType string) corev1.ServiceType {
	switch accessType {
	case ACCESS_TYPE_LOADBALANCER:
		return corev1.ServiceTypeLoadBalancer
	case ACCESS_TYPE_NODEPORT:
		return corev1.ServiceTypeNodePort
	default:
		return corev1.ServiceTypeClusterIP
	}
}

func updatePorts(spec *corev1.ServiceSpec, desired []skupperv2alpha1.SecuredAccessPort) bool {
	ports := make([]corev1.ServicePort, 0, len(desired))
	for _, p := range desired {
		port := corev1.ServicePort{Name: p.Name, Port: int32(p.Port), TargetPort: intstr.FromInt(p.TargetPort), Protocol: corev1.Protocol(p.Protocol)}
		if port.Protocol == "" {
			port.Protocol = corev1.ProtocolTCP
		}
		if port.TargetPort.IntVal == 0 {
			port.TargetPort = intstr.FromInt(p.Port)
		}
		for _, actual := range spec.Ports {
			if actual.Name == p.Name {
				port.NodePort = actual.NodePort
				port.AppProtocol = actual.AppProtocol
				break
			}
		}
		ports = append(ports, port)
	}
	if reflect.DeepEqual(spec.Ports, ports) {
		return false
	}
	spec.Ports = ports
	return true
}
