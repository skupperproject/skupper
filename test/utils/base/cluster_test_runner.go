package base

import (
	"fmt"
	vanClient "github.com/skupperproject/skupper/client"
	"gotest.tools/assert"
	"os"
	"testing"
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
	// Initialize parses flags
	Initialize(m *testing.M)
	// Initialize ClusterContexts
	Build(t *testing.T, needs ClusterNeeds, vanClientProvider VanClientProvider) []*ClusterContext
	// Return a specific public context
	GetPublicContext(id int) *ClusterContext
	// Return a specific private context
	GetPrivateContext(id int) *ClusterContext
	// Return a specific context
	GetContext(private bool, id int) *ClusterContext
}

// ClusterTestRunnerBase is a base implementation of ClusterTestRunner
type ClusterTestRunnerBase struct {
	T                 *testing.T
	Needs             ClusterNeeds
	ClusterContexts   []*ClusterContext
	vanClientProvider VanClientProvider
	unitTestMock      bool
}

// Initialize parses the command line arguments
func (c *ClusterTestRunnerBase) Initialize(m *testing.M) {
	// Parsing flags
	ParseFlags(m)
	// Running tests
	os.Exit(m.Run())
}

func (c *ClusterTestRunnerBase) Build(t *testing.T, needs ClusterNeeds, vanClientProvider VanClientProvider) []*ClusterContext {

	// Initializing internal properties
	c.T = t
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

func (c *ClusterTestRunnerBase) GetPublicContext(id int) *ClusterContext {
	return c.GetContext(false, id)
}

func (c *ClusterTestRunnerBase) GetPrivateContext(id int) *ClusterContext {
	return c.GetContext(true, id)
}

func (c *ClusterTestRunnerBase) GetContext(private bool, id int) *ClusterContext {
	if len(c.ClusterContexts) > 0 {
		for _, cc := range c.ClusterContexts {
			if cc.Private == private && cc.Id == id {
				return cc
			}
		}
	}
	c.T.Logf("requested context does not exist - id: %d - private: %v", id, private)
	return nil
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
