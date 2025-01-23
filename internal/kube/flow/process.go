package flow

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/pkg/vanflow"
)

var (
	modeExternal = "external"
	modeInternal = "internal"
)

func asProcessRecord(pod *corev1.Pod) vanflow.ProcessRecord {
	process := vanflow.ProcessRecord{
		BaseRecord: vanflow.NewBase(string(pod.ObjectMeta.UID), pod.ObjectMeta.CreationTimestamp.Time),
		Name:       &pod.ObjectMeta.Name,
		Mode:       &modeExternal,
	}
	if pod.Status.PodIP != "" {
		process.SourceHost = &pod.Status.PodIP
	}
	process.ImageName = &pod.Spec.Containers[0].Image
	process.Hostname = &pod.Spec.NodeName
	if labelName, ok := pod.ObjectMeta.Labels["app.kubernetes.io/part-of"]; ok {
		process.Group = &labelName
		if labelName == "skupper" || labelName == "skupper-network-observer" {
			process.Mode = &modeInternal
		}
	} else if labelComponent, ok := pod.ObjectMeta.Labels["app.kubernetes.io/name"]; ok {
		process.Group = &labelComponent
	} else if partOf, ok := pod.ObjectMeta.Labels["app.kubernetes.io/component"]; ok {
		process.Group = &partOf
	} else {
		// generate process group from image name
		parts := strings.Split(*process.ImageName, "/")
		part := parts[len(parts)-1]
		pg := strings.Split(part, ":")
		process.Group = &pg[0]
	}
	return process
}
