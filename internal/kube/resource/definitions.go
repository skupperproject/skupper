package resource

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func ContourHttpProxyResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "projectcontour.io",
		Version:  "v1",
		Resource: "httpproxies",
	}
}

func GatewayResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}
}

func TlsRouteResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1alpha2",
		Resource: "tlsroutes",
	}
}

func DeploymentResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
}

func IsResourceAvailable(client discovery.DiscoveryInterface, resource schema.GroupVersionResource) bool {
	resources, err := client.ServerResourcesForGroupVersion(resource.GroupVersion().String())
	if err != nil {
		return false
	}
	for _, available := range resources.APIResources {
		if available.Name == resource.Resource {
			return true
		}
	}
	return false
}
