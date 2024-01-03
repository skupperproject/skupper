package kube

import (
	"github.com/golang/glog"
	"github.com/skupperproject/skupper/api/types"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	EventRecorderRoleName        string = "event-recorder"
	EventRecorderRoleBindingName string = "event-recorder"
)

var EventRecorderPolicyRule = []rbacv1.PolicyRule{
	{
		Verbs:     []string{"watch", "create", "patch"},
		APIGroups: []string{""},
		Resources: []string{"events"},
	},
}

type SkupperEventRecorder struct {
	EventRecorder record.EventRecorder
	Source        *v1.ObjectReference
}

func (logger SkupperEventRecorder) RecordWarningEvent(reason string, message string) {
	if logger.EventRecorder != nil && logger.Source != nil {
		logger.EventRecorder.Event(logger.Source, v1.EventTypeWarning, reason, message)
	}
}

func (logger SkupperEventRecorder) RecordNormalEvent(reason string, message string) {
	if logger.EventRecorder != nil && logger.Source != nil {
		logger.EventRecorder.Event(logger.Source, v1.EventTypeNormal, reason, message)
	}
}

func NewSkupperEventRecorder(namespace string, cli kubernetes.Interface, objectRef *v1.ObjectReference) *SkupperEventRecorder {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: cli.CoreV1().Events(namespace)})
	kubeEventRecorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{
			Component: types.ControllerDeploymentName})

	eventRecorder := SkupperEventRecorder{
		EventRecorder: kubeEventRecorder,
		Source:        objectRef,
	}
	return &eventRecorder
}

func AddEventRecorderPermissions(namespace string, ownerRefs []metav1.OwnerReference, cli kubernetes.Interface, serviceAccountName string) error {
	role := rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: EventRecorderRoleName,
		},
		Rules: EventRecorderPolicyRule,
	}

	role.ObjectMeta.OwnerReferences = ownerRefs

	_, err := CreateRole(namespace, &role, cli)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	roleBinding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: EventRecorderRoleBindingName,
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: serviceAccountName,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: EventRecorderRoleName,
		},
	}

	roleBinding.ObjectMeta.OwnerReferences = ownerRefs
	_, err = CreateRoleBinding(namespace, &roleBinding, cli)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
