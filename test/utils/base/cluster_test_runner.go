package base

import (
	"context"
	"fmt"
	"log"

	"github.com/skupperproject/skupper/api/types"
	vanClient "github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/test/utils/constants"
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
	// Validate validates if given needs are based upon command line arguments
	Validate(needs ClusterNeeds) error
	// Build builds a slice of ClusterContexts to manage each participating cluster
	Build(needs ClusterNeeds, vanClientProvider VanClientProvider) ([]*ClusterContext, error)
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
}

var _ ClusterTestRunner = &ClusterTestRunnerBase{}

//
// Validate returns an error if cluster needs is not satisfied so that
// the given test suite needs to be skipped.
//
func (c *ClusterTestRunnerBase) Validate(needs ClusterNeeds) error {
	// If multiple clusters provided, see if it matches the needs
	if MultipleClusters() {
		publicAvailable := KubeConfigFilesCount(false, true)
		edgeAvailable := KubeConfigFilesCount(true, true)
		if publicAvailable < needs.PublicClusters || edgeAvailable < needs.PrivateClusters {
			// Skip if number of clusters is not enough
			return fmt.Errorf("multiple clusters provided, but this test needs %d public and %d private clusters",
				needs.PublicClusters, needs.PrivateClusters)
		}
	} else if KubeConfigFilesCount(true, true) == 0 {
		// No cluster available
		return fmt.Errorf("no cluster available")
	}

	return nil
}

//
// Build creates a ClusterContext slice prepared to communicate with all clusters
// available to the test suite.
//
func (c *ClusterTestRunnerBase) Build(needs ClusterNeeds, vanClientProvider VanClientProvider) ([]*ClusterContext, error) {

	// Initializing internal properties
	c.vanClientProvider = vanClientProvider
	c.ClusterContexts = []*ClusterContext{}

	//
	// Initializing ClusterContexts
	//
	c.Needs = needs

	// Initializing the ClusterContexts
	var err error
	if err = c.Validate(needs); err == nil {
		err = c.createClusterContexts(needs)
	}

	// Return the ClusterContext slice
	return c.ClusterContexts, err
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

func (c *ClusterTestRunnerBase) createClusterContexts(needs ClusterNeeds) error {
	if err := c.createClusterContext(needs, false); err != nil {
		return err
	}
	if err := c.createClusterContext(needs, true); err != nil {
		return err
	}
	return nil
}

func (c *ClusterTestRunnerBase) createClusterContext(needs ClusterNeeds, private bool) error {
	kubeConfigs := KubeConfigs()
	numClusters := needs.PublicClusters
	prefix := "public"
	if private {
		kubeConfigs = EdgeKubeConfigs()
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

		var vc *vanClient.VanClient
		var err error

		if c.vanClientProvider != nil {
			vc, err = c.vanClientProvider(ns, "", kubeConfig)
		} else {
			vc, err = vanClient.NewClient(ns, "", kubeConfig)
		}
		if err != nil {
			return fmt.Errorf("error initializing VanClient - %s", err)
		}

		// creating the ClusterContext
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

	return nil
}

func SetupSimplePublicPrivate(ctx context.Context, r *ClusterTestRunnerBase) error {
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

	return nil
}

func ConnectSimplePublicPrivate(ctx context.Context, r *ClusterTestRunnerBase) error {
	var err error
	pub1Cluster, err := r.GetPublicContext(1)
	if err != nil {
		return err
	}
	prv1Cluster, err := r.GetPrivateContext(1)
	if err != nil {
		return err
	}

	// Configure public cluster.
	routerCreateSpecPub := types.SiteConfigSpec{
		SkupperName:       "",
		RouterMode:        string(types.TransportModeInterior),
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     true,
		AuthMode:          types.ConsoleAuthModeInternal,
		User:              "admin",
		Password:          "admin",
		Ingress:           pub1Cluster.VanClient.GetIngressDefault(),
		Replicas:          1,
		Router:            constants.DefaultRouterOptions(nil),
	}
	routerCreateSpecPrv := types.SiteConfigSpec{
		SkupperName:       "",
		RouterMode:        string(types.TransportModeEdge),
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     true,
		AuthMode:          types.ConsoleAuthModeUnsecured,
		User:              "admin",
		Password:          "admin",
		Ingress:           pub1Cluster.VanClient.GetIngressDefault(),
		Replicas:          1,
		Router:            constants.DefaultRouterOptions(nil),
	}
	publicSiteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), routerCreateSpecPub)
	if err != nil {
		return err
	}

	err = pub1Cluster.VanClient.RouterCreate(ctx, *publicSiteConfig)
	if err != nil {
		return err
	}

	secretFile := "/tmp/" + r.Needs.NamespaceId + "_public_secret.yaml"
	err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	if err != nil {
		return err
	}

	// Configure private cluster.
	routerCreateSpecPrv.SkupperNamespace = prv1Cluster.Namespace
	privateSiteConfig, err := prv1Cluster.VanClient.SiteConfigCreate(context.Background(), routerCreateSpecPrv)

	err = prv1Cluster.VanClient.RouterCreate(ctx, *privateSiteConfig)
	if err != nil {
		return err
	}

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: prv1Cluster.Namespace,
		Name:             "public",
		Cost:             0,
	}
	_, err = prv1Cluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	return err

}

func TearDownSimplePublicAndPrivate(r *ClusterTestRunnerBase) {
	errMsg := "Something failed! aborting teardown"
	err := RemoveNamespacesForContexts(r, []int{1}, []int{1})
	if err != nil {
		log.Printf("%s: %s", errMsg, err.Error())
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

func (c *ClusterTestRunnerBase) DumpTestInfo(dirname string) {
	// Dumping info by cluster/namespace
	for _, cc := range c.ClusterContexts {
		cc.DumpTestInfo(dirname)
	}
}
