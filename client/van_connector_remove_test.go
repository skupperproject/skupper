package client

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
)

func TestConnectorRemove(t *testing.T) {
	testcases := []struct {
		doc            string
		expectedError  string
		connName       string
		createConn     bool
		secretsRemoved []string
	}{
		{
			expectedError:  "",
			doc:            "Should be able to create a connector and then remove it",
			connName:       "conn1",
			createConn:     true,
			secretsRemoved: []string{"conn1"},
		},
		{
			expectedError:  `secrets "conn1" not found`,
			doc:            "Expect remove to fail if connector was not created",
			connName:       "conn1",
			createConn:     false,
			secretsRemoved: []string{"conn1"},
		},
	}

	trans := cmp.Transformer("Sort", func(in []string) []string {
		out := append([]string(nil), in...)
		sort.Strings(out)
		return out
	})

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		secretsRemoved := []string{}

		cli, err := newMockClient("skupper", "", "")

		informers := informers.NewSharedInformerFactory(cli.KubeClient, 0)
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

		err = cli.VanRouterCreate(ctx, types.VanSiteConfig{
			Spec: types.VanSiteConfigSpec {
				SkupperName:       "skupper",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          "",
				User:              "",
				Password:          "",
				ClusterLocal:      true,
			},
		})
		assert.Check(t, err, "Unable to create VAN router")

		err = cli.VanConnectorTokenCreateFile(ctx, c.connName, testPath+c.connName+".yaml")
		assert.Check(t, err, "Unable to create connector token "+c.connName)

		if c.createConn {
			_, err = cli.VanConnectorCreateFromFile(ctx, testPath+c.connName+".yaml", types.VanConnectorCreateOptions{
				Name: c.connName,
				Cost: 1,
			})
			assert.Check(t, err, "Unable to create connector for "+c.connName)
		}

		//TODO: remove should distinguish found, not found
		err = cli.VanConnectorRemove(ctx, c.connName)
		assert.Check(t, err, "Unable to remove connector for "+c.connName)

		if c.createConn {
			time.Sleep(time.Second * 1)
			assert.Assert(t, cmp.Equal(c.secretsRemoved, secretsRemoved, trans), c.doc)
		}

		_, err = cli.VanConnectorInspect(ctx, c.connName)
		assert.Error(t, err, `secrets "`+c.connName+`" not found`, "Expect error when connector is removed")
	}

	// cleanup
	os.RemoveAll(testPath)
}
