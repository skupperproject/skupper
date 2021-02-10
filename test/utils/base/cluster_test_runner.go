package base

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/api/types"
	vanClient "github.com/skupperproject/skupper/client"
	"gotest.tools/assert"
)

// ClusterNeeds enable customization of expected number of
// public or private clusters in order to use multiple
// clusters. If number of provided clusters do not match
// test will use only 1, or will be skipped.
type ClusterNeeds struct {
	// nsId identifier that will be used to compose namespace
	NamespaceId string
	// number of public clusters expected (optional)
	PublicClusters int
	// number of private clusters expected (optional)
	PrivateClusters int
}

type VanClientProvider func(namespace string, context string, kubeConfigPath string) (*vanClient.VanClient, error)

// ClusterTestRunner defines a common interface to initialize and prepare
// tests for running against an external cluster
type ClusterTestRunner interface {
	// Initialize ClusterContexts
	BuildOrSkip(t *testing.T, needs ClusterNeeds, vanClientProvider VanClientProvider) []*ClusterContext
	// Return a specific public context
	GetPublicContext(id int) (*ClusterContext, error)
	// Return a specific private context
	GetPrivateContext(id int) (*ClusterContext, error)
	// Return a specific context
	GetContext(private bool, id int) (*ClusterContext, error)
}

// ClusterTestRunnerBase is a base implementation of ClusterTestRunner
type ClusterTestRunnerBase struct {
	Needs             ClusterNeeds
	ClusterContexts   []*ClusterContext
	vanClientProvider VanClientProvider
	unitTestMock      bool
}

var _ ClusterTestRunner = &ClusterTestRunnerBase{}

func (c *ClusterTestRunnerBase) BuildOrSkip(t *testing.T, needs ClusterNeeds, vanClientProvider VanClientProvider) []*ClusterContext {

	// Initializing internal properties
	c.vanClientProvider = vanClientProvider
	c.ClusterContexts = []*ClusterContext{}

	//
	// Initializing ClusterContexts
	//
	c.Needs = needs

	// If multiple clusters provided, see if it matches the needs
	if MultipleClusters(t) {
		publicAvailable := KubeConfigFilesCount(t, false, true)
		edgeAvailable := KubeConfigFilesCount(t, true, true)
		if publicAvailable < needs.PublicClusters || edgeAvailable < needs.PrivateClusters {
			if c.unitTestMock {
				return c.ClusterContexts
			}
			// Skip if number of clusters is not enough
			t.Skipf("multiple clusters provided, but this test needs %d public and %d private clusters",
				needs.PublicClusters, needs.PrivateClusters)
		}
	} else if KubeConfigFilesCount(t, true, true) == 0 {
		if c.unitTestMock {
			return c.ClusterContexts
		}
		// No cluster available
		t.Skipf("no cluster available")
	}

	// Initializing the ClusterContexts
	c.createClusterContexts(t, needs)

	// Return the ClusterContext slice
	return c.ClusterContexts
}

func (c *ClusterTestRunnerBase) GetPublicContext(id int) (*ClusterContext, error) {
	return c.GetContext(false, id)
}

func (c *ClusterTestRunnerBase) GetPrivateContext(id int) (*ClusterContext, error) {
	return c.GetContext(true, id)
}

func (c *ClusterTestRunnerBase) GetContext(private bool, id int) (*ClusterContext, error) {
	if len(c.ClusterContexts) > 0 {
		for _, cc := range c.ClusterContexts {
			if cc.Private == private && cc.Id == id {
				return cc, nil
			}
		}
		return nil, fmt.Errorf("ClusterContext not found")
	}
	return nil, fmt.Errorf("ClusterContexts list is empty!")
}

func (c *ClusterTestRunnerBase) createClusterContexts(t *testing.T, needs ClusterNeeds) {
	c.createClusterContext(t, needs, false)
	c.createClusterContext(t, needs, true)
}

func (c *ClusterTestRunnerBase) createClusterContext(t *testing.T, needs ClusterNeeds, private bool) {
	kubeConfigs := KubeConfigs(t)
	numClusters := needs.PublicClusters
	prefix := "public"
	if private {
		kubeConfigs = EdgeKubeConfigs(t)
		numClusters = needs.PrivateClusters
		prefix = "private"
	}

	for i := 1; i <= numClusters; i++ {
		kubeConfig := kubeConfigs[0]
		// if multiple clusters, use the appropriate one
		if len(kubeConfigs) > 1 {
			kubeConfig = kubeConfigs[i-1]
		}
		// defining the namespace to be used
		ns := fmt.Sprintf("%s-%s-%d", prefix, needs.NamespaceId, i)
		vc, err := vanClient.NewClient(ns, "", kubeConfig)
		if c.vanClientProvider != nil {
			vc, err = c.vanClientProvider(ns, "", kubeConfig)
		}
		assert.Assert(t, err, "error initializing VanClient")

		// craeting the ClusterContext
		// aca!
		cc := &ClusterContext{
			Namespace:  ns,
			KubeConfig: kubeConfig,
			VanClient:  vc,
			Private:    private,
			Id:         i,
		}

		// appending to internal slice
		c.ClusterContexts = append(c.ClusterContexts, cc)
	}

}

func SetupSimplePublicPrivateAndConnect(ctx context.Context, r *ClusterTestRunnerBase, prefix string) error {

	var err error
	pub1Cluster, err := r.GetPublicContext(1)
	if err != nil {
		return err
	}

	prv1Cluster, err := r.GetPrivateContext(1)
	if err != nil {
		return err
	}

	err = pub1Cluster.CreateNamespace()
	if err != nil {
		return err
	}

	err = prv1Cluster.CreateNamespace()
	if err != nil {
		return err
	}

	// Configure public cluster.
	routerCreateSpec := types.SiteConfigSpec{
		SkupperName:       "",
		RouterMode:        string(types.TransportModeInterior),
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     false,
		AuthMode:          types.ConsoleAuthModeUnsecured,
		User:              "nicob?",
		Password:          "nopasswordd",
		Ingress:           pub1Cluster.VanClient.GetIngressDefault(),
		Replicas:          1,
	}
	publicSiteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), routerCreateSpec)
	if err != nil {
		return err
	}

	err = pub1Cluster.VanClient.RouterCreate(ctx, *publicSiteConfig)
	if err != nil {
		return err
	}

	secretFile := "/tmp/" + prefix + "_public_secret.yaml"
	err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	if err != nil {
		return err
	}

	// Configure private cluster.
	routerCreateSpec.SkupperNamespace = prv1Cluster.Namespace
	privateSiteConfig, err := prv1Cluster.VanClient.SiteConfigCreate(context.Background(), routerCreateSpec)

	err = prv1Cluster.VanClient.RouterCreate(ctx, *privateSiteConfig)
	if err != nil {
		return err
	}

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: prv1Cluster.Namespace,
		Name:             "",
		Cost:             0,
	}
	_, err = prv1Cluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	return err

}

func TearDownSimplePublicAndPrivate(r *ClusterTestRunnerBase) {
	errMsg := "Something failed! aborting teardown"
	err := RemoveNamespacesForContexts(r, []int{1}, []int{1})
	if err != nil {
		log.Warnf("%s: %s", errMsg, err.Error())
	}
}

func RemoveNamespacesForContexts(r *ClusterTestRunnerBase, public []int, priv []int) error {
	removeNamespaces := func(private bool, ids []int) error {
		for _, id := range ids {

			cc, err := r.GetContext(private, id)
			if err != nil {
				return err
			}
			cc.DeleteNamespace()
		}
		return nil
	}
	err := removeNamespaces(true, priv)
	if err != nil {
		return err
	}
	return removeNamespaces(false, public)
}
