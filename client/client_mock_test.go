package client

import (
	"k8s.io/client-go/kubernetes/fake"
)

func newMockClient(namespace string, context string, kubeConfigPath string) (*VanClient, error) {
	return &VanClient{
		Namespace:  namespace,
		KubeClient: fake.NewSimpleClientset(),
	}, nil
}
