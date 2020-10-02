package client

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func TestConnectorRemove(t *testing.T) {
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
			connName:       "conn1",
			createConn:     true,
			secretsRemoved: []string{"conn1"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			namespace:      "van-connector-remove2",
			expectedError:  `secrets "conn1" not found`,
			doc:            "Expect remove to fail if connector was not created",
			connName:       "conn1",
			createConn:     false,
			secretsRemoved: []string{"conn1"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
	}

	var err error
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		secretsRemoved := []string{}

		// Create and set up the two namespaces that we will be using.
		tokenCreatorNamespace := c.namespace + "-token-creator"
		tokenUserNamespace := c.namespace + "-token-user"
		tokenCreatorClient, tokenUserClient := setupTwoNamespaces(t, ctx, tokenCreatorNamespace, tokenUserNamespace)
		defer kube.DeleteNamespace(tokenCreatorNamespace, tokenCreatorClient.KubeClient)
		defer kube.DeleteNamespace(tokenUserNamespace, tokenUserClient.KubeClient)

		informers := informers.NewSharedInformerFactoryWithOptions(tokenUserClient.KubeClient, 0, informers.WithNamespace(tokenUserNamespace))
		secretsInformer := informers.Core().V1().Secrets().Informer()
		secretsInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(obj interface{}) {
				secret := obj.(*corev1.Secret)
				if !strings.HasPrefix(secret.Name, "skupper") {
					secretsRemoved = append(secretsRemoved, secret.Name)
				}
			},
		})

		informers.Start(ctx.Done())
		cache.WaitForCacheSync(ctx.Done(), secretsInformer.HasSynced)

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
		assert.Check(t, err, "Unable to remove connector for "+c.connName)

		if c.createConn {
			time.Sleep(time.Second * 1)
			if diff := cmp.Diff(c.secretsRemoved, secretsRemoved, c.opts...); diff != "" {
				t.Errorf("TestConnectorRemove"+c.doc+" secrets mismatch (-want +got):\n%s", diff)
			}
		}

		_, err = tokenUserClient.ConnectorInspect(ctx, c.connName)
		assert.Error(t, err, `secrets "`+c.connName+`" not found`, "Expect error when connector is removed")
	}
}
