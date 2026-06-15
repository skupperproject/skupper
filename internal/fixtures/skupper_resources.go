package fixtures

import (
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Status(statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) skupperv2alpha1.Status {
	return skupperv2alpha1.Status{
		StatusType: statusType,
		Message:    message,
		Conditions: conditions,
	}
}

func Condition(conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

func Pod(name string, namespace string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func Service(name string, namespace string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func ServicePort(name string, port int32, targetPort int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.FromInt32(targetPort),
		Protocol:   corev1.ProtocolTCP,
	}
}

func ConfigMap(name string, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func Listener(name, namespace string) *skupperv2alpha1.Listener {
	return &skupperv2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func Site(name string, namespace string) *skupperv2alpha1.Site {
	return &skupperv2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func Connector(name string, namespace string) *skupperv2alpha1.Connector {
	return &skupperv2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func AttachedConnector(name string, namespace string) *skupperv2alpha1.AttachedConnector {
	return &skupperv2alpha1.AttachedConnector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AttachedConnector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func AttachedConnectorBinding(name string, namespace string) *skupperv2alpha1.AttachedConnectorBinding {
	return &skupperv2alpha1.AttachedConnectorBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AttachedConnectorBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func MultiKeyListener(name string, namespace string) *skupperv2alpha1.MultiKeyListener {
	return &skupperv2alpha1.MultiKeyListener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "MultiKeyListener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func RouterAccess(name string, namespace string) *skupperv2alpha1.RouterAccess {
	return &skupperv2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func SecuredAccess(name string, namespace string) *skupperv2alpha1.SecuredAccess {
	return &skupperv2alpha1.SecuredAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "SecuredAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
