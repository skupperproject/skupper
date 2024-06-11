package flow

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/kube"
)

type HostUpdateHandler func(deleted bool, name string, process *flow.HostRecord) error

func WatchNodes(controller *kube.Controller, namespace string, handler HostUpdateHandler) {
	controller.WatchNodes(func(key string, node *corev1.Node) error {
		if node == nil {
			handler(true, key, nil)
		} else {
			handler(false, key, AsHostRecord("-"+namespace, node))
		}
		return nil
	})
}

func AsHostRecord(qualifier string, node *corev1.Node) *flow.HostRecord {
	host := &flow.HostRecord{}
	// Note: skupper running in multiple ns in same cluster the hosts are the same
	host.Identity = string(node.ObjectMeta.UID) + qualifier
	host.StartTime = uint64(node.ObjectMeta.CreationTimestamp.UnixNano()) / uint64(time.Microsecond)
	host.Name = &node.ObjectMeta.Name
	host.Arch = &node.Status.NodeInfo.Architecture

	provider := strings.Split(node.Spec.ProviderID, "://")
	if len(provider) > 0 {
		host.Provider = &provider[0]
	}
	if region, ok := node.ObjectMeta.Labels["topology.kubernetes.io/region"]; ok {
		host.Location = &region
		host.Region = &region
	}
	if zone, ok := node.ObjectMeta.Labels["topology.kubernetes.io/zone"]; ok {
		host.Zone = &zone
	}
	host.ContainerRuntime = &node.Status.NodeInfo.ContainerRuntimeVersion
	host.KernelVersion = &node.Status.NodeInfo.KernelVersion
	host.KubeProxyVersion = &node.Status.NodeInfo.KubeProxyVersion
	host.KubeletVersion = &node.Status.NodeInfo.KubeletVersion
	return host
}
