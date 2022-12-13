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

type SkupperEventRecorder struct {
	EventRecorder record.EventRecorder
	Source        *v1.Service
}

func NewEventRecorder(namespace string, cli kubernetes.Interface) SkupperEventRecorder {

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
	}
	return eventRecorder
}

func RecordWarningEvent(namespace string, reason string, message string, recorder SkupperEventRecorder, cli kubernetes.Interface) {
	event.Recordf(reason, message)
	recorder.EventRecorder.Event(recorder.Source, v1.EventTypeWarning, reason, message)
}

func RecordNormalEvent(namespace string, reason string, message string, recorder SkupperEventRecorder, cli kubernetes.Interface) {
	event.Recordf(reason, message)
	recorder.EventRecorder.Event(recorder.Source, v1.EventTypeNormal, reason, message)
}
