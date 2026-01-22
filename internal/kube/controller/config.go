package controller

import (
	"flag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	iflag "github.com/skupperproject/skupper/internal/flag"
	"github.com/skupperproject/skupper/internal/kube/grants"
	"github.com/skupperproject/skupper/internal/kube/metrics"
	"github.com/skupperproject/skupper/internal/kube/securedaccess"
)

type Config struct {
	GrantConfig            *grants.GrantConfig
	SecuredAccessConfig    *securedaccess.Config
	MetricsConfig          *metrics.Config
	Namespace              string
	Kubeconfig             string
	WatchNamespace         string
	Name                   string
	RequireExplicitControl bool
	DisableSecurityContext bool
}

func (c *Config) WatchingAllNamespaces() bool {
	return c.WatchNamespace == metav1.NamespaceAll
}

func (c *Config) requireExplicitControl() bool {
	return !c.WatchingAllNamespaces() || c.RequireExplicitControl
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
	metricsConfig, err := metrics.BoundConfig(flags)
	if err != nil {
		return nil, err
	}
	c := &Config{
		GrantConfig:         grantConfig,
		SecuredAccessConfig: securedAccessConfig,
		MetricsConfig:       metricsConfig,
	}
	iflag.StringVar(flags, &c.Namespace, "namespace", "NAMESPACE", "", "The Kubernetes namespace scope for the controller")
	iflag.StringVar(flags, &c.Kubeconfig, "kubeconfig", "KUBECONFIG", "", "A path to the kubeconfig file to use")
	iflag.StringVar(flags, &c.WatchNamespace, "watch-namespace", "WATCH_NAMESPACE", metav1.NamespaceAll, "The Kubernetes namespace the controller should monitor for controlled resources (will monitor all if not specified)")
	iflag.StringVar(flags, &c.Name, "name", "CONTROLLER_NAME", "", "A name identifying the controller. If not specified it will be deduced from the hostname.")
	iflag.BoolVar(flags, &c.RequireExplicitControl, "require-explicit-control", "REQUIRE_EXPLICIT_CONTROL", false, "If set, this controller instance will only process resources in which there is a ConfigMap named skupper with an entry 'controller' whose value matches the controller's namespace qualified name. Controllers watching a single namespace require that ConfigMap regardless of this setting.")
	iflag.BoolVar(flags, &c.DisableSecurityContext, "disable-security-context", "DISABLE_SECURITY_CONTEXT", false, "If set, the default security context definitions won't be set to the skupper-router deployment's pod and containers.")
	return c, nil
}
