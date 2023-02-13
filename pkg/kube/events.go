package kube

import (
	"github.com/golang/glog"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
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
	Source        *v1.Service
	Disabled      bool
}

func NewEventRecorder(namespace string, cli kubernetes.Interface, disabled bool) SkupperEventRecorder {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: cli.CoreV1().Events(namespace)})
	kubeEventRecorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{
			Component: types.ControllerServiceName})
	service, _ := cli.CoreV1().Services(namespace).Get(types.ControllerServiceName, metav1.GetOptions{})

	eventRecorder := SkupperEventRecorder{
		EventRecorder: kubeEventRecorder,
		Source:        service,
		Disabled:      disabled,
	}
	return eventRecorder
}

func RecordWarningEvent(reason string, message string, recorder SkupperEventRecorder) {
	event.Recordf(reason, message)
	if !recorder.Disabled && recorder.EventRecorder != nil && recorder.Source != nil {
		recorder.EventRecorder.Event(recorder.Source, v1.EventTypeWarning, reason, message)
	}
}

func RecordNormalEvent(reason string, message string, recorder SkupperEventRecorder) {
	event.Recordf(reason, message)
	if !recorder.Disabled && recorder.EventRecorder != nil && recorder.Source != nil {
		recorder.EventRecorder.Event(recorder.Source, v1.EventTypeNormal, reason, message)
	}
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
	_, err = CreateRoleBinding(NamespaceFile, &roleBinding, cli)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
