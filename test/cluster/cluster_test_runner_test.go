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

//TODO implementme
//func verifyNamespaces(t *testing.T, ns NamespacesInterface, expected_namespaces []string)

//add test of dns valid namespace
func TestClusterContextNamespaceCreationDeletion(t *testing.T) {
	var err error
	ns_prefix := "ns-prefix"
	kc := "someconfig"
	//ctx := context.Background()

	cc := BuildClusterContext(t, ns_prefix, kc, NewMockClient)
	NamespacesClient := cc.VanClient.KubeClient.CoreV1().Namespaces()

	assert.Equal(t, "", cc.CurrentNamespace)
	assert.Equal(t, 0, len(cc.Namespaces))

	err = cc.CreateNamespace()
	t.Logf("curr namespace = %v\n", cc.CurrentNamespace)
	assert.Equal(t, ns_prefix+"-1", cc.CurrentNamespace)
	assert.Equal(t, 1, len(cc.Namespaces))
	assert.Assert(t, err)

	list, err := NamespacesClient.List(metav1.ListOptions{})
	assert.Assert(t, err)
	assert.Equal(t, 1, len(list.Items))
	assert.Equal(t, ns_prefix+"-1", list.Items[0].Name)

	err = cc.CreateNamespace()
	assert.Equal(t, ns_prefix+"-2", cc.CurrentNamespace)
	assert.Equal(t, 2, len(cc.Namespaces))
	assert.Assert(t, err)

	list, err = NamespacesClient.List(metav1.ListOptions{})
	assert.Assert(t, err)
	assert.Equal(t, 2, len(list.Items))
	assert.Equal(t, ns_prefix+"-1", list.Items[0].Name)
	assert.Equal(t, ns_prefix+"-2", list.Items[1].Name)

	cc.DeleteNamespaces()
	//TODO verify the namespace exists in the clientmock

	assert.Equal(t, "", cc.CurrentNamespace)
	assert.Equal(t, 0, len(cc.Namespaces))

	list, err = NamespacesClient.List(metav1.ListOptions{})
	assert.Assert(t, err)
	assert.Equal(t, 0, len(list.Items))

	NsSpec := &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns_prefix + "-1"}}
	_, err = cc.VanClient.KubeClient.CoreV1().Namespaces().Create(NsSpec)
	assert.Assert(t, err)

	err = cc.CreateNamespace()
	assert.Error(t, err, "namespaces \"ns-prefix-1\" already exists")
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
	//var err error
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
	//var err error
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
