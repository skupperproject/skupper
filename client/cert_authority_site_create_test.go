package client

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
	"testing"
)

func TestCertAuthoritySiteCreateDefaults(t *testing.T) {
	testcases := []struct {
		doc             string
		namespace       string
		expectedError   string
		skupperName     string
		siteUID         string
		opts            []cmp.Option
		secretsExpected []string
	}{
		{
			namespace:     "van-ca-site-create1",
			expectedError: "",
			doc:           "The certificate authority is created successfully.",
			skupperName:   "skupper-ca-test-site",
			siteUID:       "dc9076e9-2fda-4019-bd2c-900a8284b9c4",
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "skupper") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "dockercfg") }),
				cmpopts.IgnoreSliceElements(func(v string) bool { return strings.Contains(v, "token") }),
			},
			secretsExpected: []string{types.SiteCaSecret},
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

		err = cli.CASiteCreate(types.SiteConfig{
			Spec: types.SiteConfigSpec{
				SkupperName: c.skupperName,
			},
			Reference: types.SiteConfigReference{
				UID: c.siteUID,
			},
		})

		assert.Check(t, err, c.doc)

		secret, _ := cli.KubeClient.CoreV1().Secrets(c.namespace).Get(c.siteUID, metav1.GetOptions{})

		assert.Check(t, secret.Name == c.siteUID)

	}
}
