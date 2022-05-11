package client

import (
	"time"

	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/skupperproject/skupper/api/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/skupperproject/skupper/pkg/kube"
)

var Version = "undefined"
var minimumCompatibleVersion = "0.8.0"
var defaultRetry = wait.Backoff{
	Steps:    100,
	Duration: 10 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

// A VAN Client manages orchestration and communications with the network components
type VanClient struct {
	Namespace     string
	KubeClient    kubernetes.Interface
	RouteClient   *routev1client.RouteV1Client
	RestConfig    *restclient.Config
	DynamicClient dynamic.Interface
}

func (cli *VanClient) GetNamespace() string {
	return cli.Namespace
}

func (cli *VanClient) GetKubeClient() kubernetes.Interface {
	return cli.KubeClient
}

func (cli *VanClient) GetVersion(component string, name string) string {
	return kube.GetComponentVersion(cli.Namespace, cli.KubeClient, component, name)
}

func (cli *VanClient) GetMinimumCompatibleVersion() string {
	return minimumCompatibleVersion
}

func NewClient(namespace string, context string, kubeConfigPath string) (*VanClient, error) {
	c := &VanClient{}

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
	c.RestConfig = restconfig
	c.KubeClient, err = kubernetes.NewForConfig(restconfig)
	if err != nil {
		return c, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(restconfig)
	resources, err := dc.ServerResourcesForGroupVersion("route.openshift.io/v1")
	if err == nil && len(resources.APIResources) > 0 {
		c.RouteClient, err = routev1client.NewForConfig(restconfig)
		if err != nil {
			return c, err
		}
	}

	if namespace == "" {
		c.Namespace, _, err = kubeconfig.Namespace()
		if err != nil {
			return c, err
		}
	} else {
		c.Namespace = namespace
	}
	c.DynamicClient, err = dynamic.NewForConfig(restconfig)
	if err != nil {
		return c, err
	}

	return c, nil
}

func (cli *VanClient) GetIngressDefault() string {
	if cli.RouteClient == nil {
		return types.IngressLoadBalancerString
	}
	return types.IngressRouteString
}
