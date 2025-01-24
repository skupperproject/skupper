package controller

import (
	"flag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	iflag "github.com/skupperproject/skupper/internal/flag"
	"github.com/skupperproject/skupper/internal/kube/grants"
	"github.com/skupperproject/skupper/internal/kube/securedaccess"
)

type Config struct {
	GrantConfig         *grants.GrantConfig
	SecuredAccessConfig *securedaccess.Config
	Namespace           string
	Kubeconfig          string
	WatchNamespace      string
	Name                string
	RequireAnnotation   bool
}

func (c *Config) WatchingAllNamespaces() bool {
	return c.WatchNamespace == metav1.NamespaceAll
}

func (c *Config) sitesRequireAnnotation() bool {
	return !c.WatchingAllNamespaces() || c.RequireAnnotation
}

func BoundConfig(flags *flag.FlagSet) (*Config, error) {
	grantConfig, err := grants.BoundGrantConfig(flags)
	if err != nil {
		return nil, err
	}
	securedAccessConfig, err := securedaccess.BoundConfig(flags)
	if err != nil {
		return nil, err
	} else if err := securedAccessConfig.Verify(); err != nil {
		return nil, err
	}
	c := &Config{
		GrantConfig:         grantConfig,
		SecuredAccessConfig: securedAccessConfig,
	}
	iflag.StringVar(flags, &c.Namespace, "namespace", "NAMESPACE", "", "The Kubernetes namespace scope for the controller")
	iflag.StringVar(flags, &c.Kubeconfig, "kubeconfig", "KUBECONFIG", "", "A path to the kubeconfig file to use")
	iflag.StringVar(flags, &c.WatchNamespace, "watch-namespace", "WATCH_NAMESPACE", metav1.NamespaceAll, "The Kubernetes namespace the controller should monitor for controlled resources (will monitor all if not specified)")
	iflag.StringVar(flags, &c.Name, "name", "CONTROLLER_NAME", "", "A name identifying the controller. If mot specified it will be deduced from the hostname.")
	iflag.BoolVar(flags, &c.RequireAnnotation, "sites-require-annotation", "SITES_REQUIRE_ANNOTATION", false, "If set, this controller instance will only process sites with an annotation matching the controller's namespace qualified name")
	return c, nil
}
