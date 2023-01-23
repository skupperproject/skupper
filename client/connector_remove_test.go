package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func TestConnectorRemove(t *testing.T) {
	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
		return
	}

	testcases := []struct {
		namespace      string
		doc            string
		expectedError  string
		connName       string
		createConn     bool
		secretsRemoved []string
		opts           []cmp.Option
	}{
		{
			namespace:      "van-connector-remove1",
			expectedError:  "",
			doc:            "Should be able to create a connector and then remove it",
			connName:       "link1",
			createConn:     true,
			secretsRemoved: []string{"link1"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "link") }),
			},
		},
		{
			namespace:      "van-connector-remove2",
			expectedError:  `No such link "link1"`,
			doc:            "Expect remove to fail if connector was not created",
			connName:       "link1",
			createConn:     false,
			secretsRemoved: nil,
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "link") }),
			},
		},
	}

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create and set up the two namespaces that we will be using.
		tokenCreatorNamespace := c.namespace + "-token-creator"
		tokenUserNamespace := c.namespace + "-token-user"
		tokenCreatorClient, tokenUserClient, err := setupTwoNamespaces(t, ctx, tokenCreatorNamespace, tokenUserNamespace)
		assert.Assert(t, err, "Can't set up namespaces")
		defer kube.DeleteNamespace(tokenCreatorNamespace, tokenCreatorClient.KubeClient)
		defer kube.DeleteNamespace(tokenUserNamespace, tokenUserClient.KubeClient)

		err = tokenCreatorClient.ConnectorTokenCreateFile(ctx, c.connName, testPath+c.connName+".yaml")
		assert.Check(t, err, "Unable to create connector token "+c.connName)

		if c.createConn {
			_, err = tokenUserClient.ConnectorCreateFromFile(ctx, testPath+c.connName+".yaml", types.ConnectorCreateOptions{
				Name:             c.connName,
				SkupperNamespace: tokenUserNamespace,
				Cost:             1,
			})
			assert.Check(t, err, "Unable to create connector for "+c.connName)
		}

		//TODO: remove should distinguish found, not found
		err = tokenUserClient.ConnectorRemove(ctx, types.ConnectorRemoveOptions{
			Name:             c.connName,
			SkupperNamespace: tokenUserNamespace,
			ForceCurrent:     false,
		})
		for i := 0; i < 5 && k8serrors.IsConflict(errors.Unwrap(err)); i++ {
			time.Sleep(500 * time.Millisecond)
			err = tokenUserClient.ConnectorRemove(ctx, types.ConnectorRemoveOptions{
				Name:             c.connName,
				SkupperNamespace: tokenUserNamespace,
				ForceCurrent:     false,
			})
		}
		if c.expectedError == "" {
			assert.Check(t, err, "Unable to remove connector for "+c.connName+" "+c.namespace)
		} else {
			assert.Error(t, err, c.expectedError, c.namespace)
		}

		for _, name := range c.secretsRemoved {
			_, err := tokenUserClient.KubeClient.CoreV1().Secrets(c.namespace).Get(context.TODO(), name, metav1.GetOptions{})
			assert.Assert(t, k8serrors.IsNotFound(err), c.namespace)
		}

		_, err = tokenUserClient.ConnectorInspect(ctx, c.connName)
		assert.Error(t, err, `secrets "`+c.connName+`" not found`, "Expect error when connector is removed")
	}
}
