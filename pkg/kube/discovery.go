package kube

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

var ingressResources = [][]schema.GroupVersionResource {
	{
		{
			Group:    "projectcontour.io",
			Version:  "v1",
			Resource: "httpproxies",
		},
	},
	{
		{
			Group:    "route.openshift.io",
			Version:  "v1",
			Resource: "routes",
		},
	},
	{
		{
			Group:    "networking.k8s.io",
			Version:  "v1",
			Resource: "ingresses",
		},
		{
			Group:    "networking.k8s.io",
			Version:  "v1beta1",
			Resource: "ingresses",
		},
	},
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

func GetSupportedIngressResources(client discovery.DiscoveryInterface) []schema.GroupVersionResource {
	if client == nil {
		return nil
	}
	var valid []schema.GroupVersionResource
	for _, resources := range ingressResources {
		for _, resource := range resources {
			if IsResourceAvailable(client, resource) {
				valid = append(valid, resource)
			}
		}
	}
	return valid
}
