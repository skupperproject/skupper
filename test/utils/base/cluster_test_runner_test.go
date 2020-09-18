package base

import (
	"testing"

	"github.com/skupperproject/skupper/client"
	"gotest.tools/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuild(t *testing.T) {
	runner := &ClusterTestRunnerBase{}
	// only set this to true when running unit test
	runner.unitTestMock = true

	tcs := []struct {
		name             string
		public           int
		private          int
		publicNeeded     int
		privateNeeded    int
		expectedContexts int
	}{
		{"multiple-cluster-needs-satisfied", 3, 2, 3, 2, 5},
		{"multiple-cluster-needs-not-satisfied", 3, 2, 5, 2, 0},
		{"single-cluster-needs-satisfied", 1, 0, 1, 1, 2},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			setUnitTestFlags(tc.public, tc.private)
			contexts := runner.BuildOrSkip(t, ClusterNeeds{
				NamespaceId:     "unit-test",
				PublicClusters:  tc.publicNeeded,
				PrivateClusters: tc.privateNeeded,
			}, func(namespace string, context string, kubeConfigPath string) (*client.VanClient, error) {
				return &client.VanClient{
					Namespace:  namespace,
					KubeClient: fake.NewSimpleClientset(),
				}, nil
			})
			assert.Equal(t, len(contexts), tc.expectedContexts)
		})
	}
}

func TestGetContext(t *testing.T) {
	notFoundError := "ClusterContext not found"
	c := &ClusterTestRunnerBase{}
	ccPublic := ClusterContext{
		Private: false,
		Id:      22,
	}
	ccPrivate := ClusterContext{
		Private: true,
		Id:      22,
	}

	cc, err := c.GetContext(true, 1)
	assert.Error(t, err, "ClusterContexts list is empty!")
	assert.Assert(t, cc == nil)

	c.ClusterContexts = []*ClusterContext{&ccPublic}

	cc, err = c.GetContext(true, 1)
	assert.Error(t, err, notFoundError)

	cc, err = c.GetContext(false, 1)
	assert.Error(t, err, notFoundError)

	cc, err = c.GetContext(true, 22)
	assert.Error(t, err, notFoundError)

	cc, err = c.GetContext(false, 22)
	assert.Assert(t, err)
	assert.Assert(t, &ccPublic == cc)

	c.ClusterContexts = []*ClusterContext{&ccPrivate}

	cc, err = c.GetContext(true, 1)
	assert.Error(t, err, notFoundError)

	cc, err = c.GetContext(false, 1)
	assert.Error(t, err, notFoundError)

	cc, err = c.GetContext(false, 22)
	assert.Error(t, err, notFoundError)

	cc, err = c.GetContext(true, 22)
	assert.Assert(t, err)
	assert.Assert(t, &ccPrivate == cc)
}
