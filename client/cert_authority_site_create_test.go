package client

import (
	"context"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"testing"
)

func TestCertAuthoritySiteCreate(t *testing.T) {
	testcases := []struct {
		doc           string
		namespace     string
		expectedError string
		skupperName   string
		siteUID       string
		secret        string
	}{
		{
			namespace:     "van-ca-site-create1",
			expectedError: "",
			doc:           "The certificate authority is created successfully.",
			skupperName:   "test-site",
			siteUID:       "dc9076e9",
			secret:        "skupper-site-ca-services",
		},
	}

	isCluster := *clusterRun

	for _, c := range testcases {
		_, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create the client
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(c.namespace, "", "")
		} else {
			cli, err = NewClient(c.namespace, "", "")
		}
		assert.Check(t, err, c.doc)

		_, err = kube.NewNamespace(c.namespace, cli.KubeClient)
		assert.Check(t, err, c.doc)
		defer func(name string, cli kubernetes.Interface) {
			err := kube.DeleteNamespace(name, cli)
			if err != nil {

			}
		}(c.namespace, cli.KubeClient)

		ownerReference := metav1.OwnerReference{
			APIVersion: "core/v1",
			Kind:       "ConfigMap",
			Name:       c.skupperName,
			UID:        types.UID(c.siteUID),
		}
		err = cli.CASiteCreate(&ownerReference)

		assert.Check(t, err, c.doc)

		secret, _ := cli.KubeClient.CoreV1().Secrets(c.namespace).Get(c.secret, metav1.GetOptions{})

		assert.Check(t, secret != nil, "Secret "+c.secret+" has not been created.")

	}
}
