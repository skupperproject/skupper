package fake

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	routefake "github.com/openshift/client-go/route/clientset/versioned/fake"
	"github.com/skupperproject/skupper/internal/kube/client"
	skupperclientfake "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/fake"
	fakeskupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	testing "k8s.io/client-go/testing"
)

func NewFakeClient(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*client.KubeClient, error) {
	c := &client.KubeClient{}

	c.Namespace = namespace
	c.Kube = k8sfake.NewSimpleClientset(k8sObjects...)
	c.Skupper = skupperclientfake.NewSimpleClientset(skupperObjects...)
	// Note: brute force error return for any client access, we could make it more granular if needed
	if fakeSkupperError != "" {
		c.Skupper.SkupperV1alpha1().(*fakeskupperv1alpha1.FakeSkupperV1alpha1).PrependReactor("*", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf(fakeSkupperError)
		})
	}
	// todo populuate scheme
	//	s := FakeSkupperScheme
	//	c.DynamicClient = dynamicfake.NewSimpleDynamicClient(s)
	c.Dynamic = nil
	c.Discovery = c.Skupper.Discovery()
	c.Route = routefake.NewSimpleClientset()

	return c, nil
}
