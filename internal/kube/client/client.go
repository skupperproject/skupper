// Package client provides access to various APIS used to interact
// with the Kubernetes API server.
package client

import (
	openshiftapps "github.com/openshift/client-go/apps/clientset/versioned"
	openshiftroute "github.com/openshift/client-go/route/clientset/versioned"

	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	skupperclient "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned"
)

// The Clients interface defines acess to different types of client
// interface required for interactions withe the Kubernetes API
// server.
type Clients interface {
	GetKubeClient() kubernetes.Interface
	GetDynamicClient() dynamic.Interface
	GetDiscoveryClient() discovery.DiscoveryInterface
	GetRouteInterface() openshiftroute.Interface
	GetRouteClient() routev1client.RouteV1Interface
	GetSkupperClient() skupperclient.Interface
}

// A Kube Client manages orchestration and communications with the network components
type KubeClient struct {
	Namespace string
	Kube      kubernetes.Interface
	Route     openshiftroute.Interface
	//RouteClient     *routev1client.RouteV1Client
	OCApps    openshiftapps.Interface
	Rest      *restclient.Config
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
	Skupper   skupperclient.Interface
}

func (c *KubeClient) GetNamespace() string {
	return c.Namespace
}

func (c *KubeClient) GetKubeClient() kubernetes.Interface {
	return c.Kube
}

func (c *KubeClient) GetDynamicClient() dynamic.Interface {
	return c.Dynamic
}

func (c *KubeClient) GetDiscoveryClient() discovery.DiscoveryInterface {
	return c.Discovery
}

func (c *KubeClient) GetRouteInterface() openshiftroute.Interface {
	return c.Route
}

func (c *KubeClient) GetRouteClient() routev1client.RouteV1Interface {
	return c.Route.RouteV1()
}

func (c *KubeClient) GetSkupperClient() skupperclient.Interface {
	return c.Skupper
}

func NewClient(namespace string, context string, kubeConfigPath string) (*KubeClient, error) {
	c := &KubeClient{}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeConfigPath != "" {
		loadingRules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}
	}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		},
	)
	restconfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return c, err
	}
	restconfig.ContentConfig.GroupVersion = &schema.GroupVersion{Version: "v1"}
	restconfig.APIPath = "/api"
	restconfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	c.Rest = restconfig
	c.Kube, err = kubernetes.NewForConfig(restconfig)
	if err != nil {
		return c, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(restconfig)
	if err != nil {
		return c, err
	}
	resources, err := dc.ServerResourcesForGroupVersion("route.openshift.io/v1")
	if err == nil && len(resources.APIResources) > 0 {
		c.Route, err = openshiftroute.NewForConfig(restconfig)
		//c.RouteClient, err = routev1client.NewForConfig(restconfig)
		if err != nil {
			return c, err
		}
	}
	resources, err = dc.ServerResourcesForGroupVersion("apps.openshift.io/v1")
	if err == nil && len(resources.APIResources) > 0 {
		c.OCApps, err = openshiftapps.NewForConfig(restconfig)
		if err != nil {
			return c, err
		}
	}

	c.Discovery = dc

	if namespace == "" {
		c.Namespace, _, err = kubeconfig.Namespace()
		if err != nil {
			return c, err
		}
	} else {
		c.Namespace = namespace
	}
	c.Dynamic, err = dynamic.NewForConfig(restconfig)
	if err != nil {
		return c, err
	}
	c.Skupper, err = skupperclient.NewForConfig(restconfig)
	if err != nil {
		return c, err
	}

	return c, nil
}
