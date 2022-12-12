package kube

import (
	"github.com/golang/glog"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

func NewEventRecorder(namespace string, cli kubernetes.Interface) record.EventRecorder {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: cli.CoreV1().Events(namespace)})
	eventRecorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{
			Component: types.ControllerServiceName})
	return eventRecorder
}

func RecordWarningEvent(namespace string, reason string, message string, recorder record.EventRecorder, cli kubernetes.Interface) {
	event.Recordf(reason, message)
	deployment, _ := cli.CoreV1().Services(namespace).Get(types.ControllerServiceName, metav1.GetOptions{})
	recorder.Event(deployment, v1.EventTypeWarning, reason, message)
}

func RecordNormalEvent(namespace string, reason string, message string, recorder record.EventRecorder, cli kubernetes.Interface) {
	event.Recordf(reason, message)
	deployment, _ := cli.CoreV1().Services(namespace).Get(types.ControllerServiceName, metav1.GetOptions{})
	recorder.Event(deployment, v1.EventTypeNormal, reason, message)
}
