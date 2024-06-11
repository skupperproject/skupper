package flow

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/kube"
)

type ProcessUpdateHandler func(deleted bool, name string, process *flow.ProcessRecord) error

func WatchPods(controller *kube.Controller, namespace string, handler ProcessUpdateHandler) {
	controller.WatchAllPods(namespace, func(key string, pod *corev1.Pod) error {
		if pod == nil {
			handler(true, key, nil)
		} else {
			handler(false, key, AsProcessRecord(pod))
		}
		return nil
	})
}

func AsProcessRecord(pod *corev1.Pod) *flow.ProcessRecord {
	process := &flow.ProcessRecord{}
	process.Identity = string(pod.ObjectMeta.UID)
	process.StartTime = uint64(pod.ObjectMeta.CreationTimestamp.UnixNano()) / uint64(time.Microsecond)
	process.Name = &pod.ObjectMeta.Name
	if pod.Status.PodIP != "" {
		process.SourceHost = &pod.Status.PodIP
	}
	process.Image = &pod.Spec.Containers[0].Image
	process.ImageName = process.Image
	process.HostName = &pod.Spec.NodeName
	process.ProcessRole = &flow.External
	if labelName, ok := pod.ObjectMeta.Labels["app.kubernetes.io/part-of"]; ok {
		process.GroupName = &labelName
		if labelName == "skupper" {
			process.ProcessRole = &flow.Internal
		}
	} else if labelComponent, ok := pod.ObjectMeta.Labels["app.kubernetes.io/name"]; ok {
		process.GroupName = &labelComponent
	} else if partOf, ok := pod.ObjectMeta.Labels["app.kubernetes.io/component"]; ok {
		process.GroupName = &partOf
	} else {
		// generate process group from image name
		parts := strings.Split(*process.ImageName, "/")
		part := parts[len(parts)-1]
		pg := strings.Split(part, ":")
		process.GroupName = &pg[0]
	}
	return process
}
