package base

import (
	"github.com/skupperproject/skupper/client"
	"gotest.tools/assert"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestBuild(t *testing.T) {
	var runner ClusterTestRunner
	runner = &ClusterTestRunnerBase{}
	// only set this to true when running unit test
	runner.(*ClusterTestRunnerBase).unitTestMock = true

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
			contexts := runner.Build(t, ClusterNeeds{
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
