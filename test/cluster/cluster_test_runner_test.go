package cluster

import (
	"testing"

	"github.com/skupperproject/skupper/client"
	"gotest.tools/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func NewMockClient(namespace string, context string, kubeConfigPath string) (*client.VanClient, error) {
	return &client.VanClient{
		Namespace:  namespace,
		KubeClient: fake.NewSimpleClientset(),
	}, nil
}

//TODO add test of dns valid namespace i.e: - are not allowed in namespace name
func TestClusterContextNamespaceCreationDeletion(t *testing.T) {
	var err error
	ns_prefix := "ns-prefix"
	kc := "someconfig"

	cc := BuildClusterContext(t, ns_prefix, kc, NewMockClient)
	NamespacesClient := cc.VanClient.KubeClient.CoreV1().Namespaces()

	verifyNamespacesInCluster := func(expected_namespaces []string) {
		//this only works in a mocked cluster where the only existing
		//namespaces are those created by this test.
		t.Helper()
		list, err := NamespacesClient.List(metav1.ListOptions{})
		assert.Assert(t, err)
		assert.Equal(t, len(expected_namespaces), len(list.Items))
		for i, ns := range list.Items {
			assert.Equal(t, ns.Name, expected_namespaces[i])
		}
	}

	verifyCurrentAndCount := func(expected_current string, count int) {
		t.Helper()
		assert.Equal(t, expected_current, cc.CurrentNamespace)
		assert.Equal(t, count, len(cc.Namespaces))
	}

	verifyCurrentAndCount("", 0)

	err = cc.CreateNamespace()
	assert.Assert(t, err)

	verifyCurrentAndCount(ns_prefix+"-1", 1)
	verifyNamespacesInCluster([]string{ns_prefix + "-1"})

	err = cc.CreateNamespace()
	assert.Assert(t, err)

	verifyCurrentAndCount(ns_prefix+"-2", 2)
	verifyNamespacesInCluster([]string{ns_prefix + "-1", ns_prefix + "-2"})

	cc.DeleteNamespaces()

	verifyCurrentAndCount("", 0)
	verifyNamespacesInCluster([]string{})
}

func TestClusterContextNamespaceAlreadyExists(t *testing.T) {
	var err error
	ns_prefix := "ns-prefix"
	kc := "someconfig"

	cc := BuildClusterContext(t, ns_prefix, kc, NewMockClient)

	NsSpec := &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns_prefix + "-1"}}
	_, err = cc.VanClient.KubeClient.CoreV1().Namespaces().Create(NsSpec)
	assert.Assert(t, err)

	err = cc.CreateNamespace()
	assert.Error(t, err, "namespaces \"ns-prefix-1\" already exists")
}

func Test_getNextNamespace(t *testing.T) {
	prefix := "prefix"
	cc := &ClusterContext{}
	cc.NamespacePrefix = prefix

	//getNextNamespace does not update the list, just tells you what would
	//be the next
	next := cc.getNextNamespace()
	assert.Equal(t, next, prefix+"-1")
	assert.Equal(t, cc.getNextNamespace(), prefix+"-1")

	assert.Equal(t, len(cc.Namespaces), 0)
	assert.Equal(t, cc.CurrentNamespace, "")
}

func Test_moveToNextNamespace(t *testing.T) {
	prefix := "prefix"
	cc := BuildClusterContext(t, prefix, "someconfig", NewMockClient)
	cc.NamespacePrefix = prefix

	cc.moveToNextNamespace()
	assert.Equal(t, cc.CurrentNamespace, prefix+"-1")
	assert.Equal(t, len(cc.Namespaces), 1)
	cc.Namespaces[0] = prefix + "-1"

	cc.moveToNextNamespace()
	assert.Equal(t, cc.CurrentNamespace, prefix+"-2")
	assert.Equal(t, len(cc.Namespaces), 2)
	cc.Namespaces[0] = prefix + "-1"
	cc.Namespaces[1] = prefix + "-2"
}
