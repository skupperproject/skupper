package client

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func TestConnectorCreateError(t *testing.T) {
	cli, err := newMockClient("skupper", "", "")
	assert.Check(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = cli.VanConnectorCreate(ctx, "./somefile.yaml", types.VanConnectorCreateOptions{
		Name: "",
		Cost: 1,
	})
	assert.Error(t, err, "open ./somefile.yaml: no such file or directory", "Expect error when file not found")
}

func TestConnectorCreateInterior(t *testing.T) {
	testcases := []struct {
		doc                string
		expectedError      string
		connName           string
		connFile           string
		secretExpectedName string
	}{
		{
			doc:                "Expect generated name to be conn1",
			expectedError:      "",
			connName:           "",
			secretExpectedName: "conn1",
		},
		{
			doc:                "Expect secret name to be as provided: conn22",
			expectedError:      "",
			connName:           "conn2",
			secretExpectedName: "conn2",
		},
	}

	//TODO do a symple loop verifying and asserting no repeated table
	//connection.

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("skupper", "", "")
	assert.Check(t, err)

	secrets := make(chan *corev1.Secret)

	informers := informers.NewSharedInformerFactory(cli.KubeClient, 0)
	secretsInformer := informers.Core().V1().Secrets().Informer()
	secretsInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*corev1.Secret)
			t.Logf("secret added: %s/%s", secret.Namespace, secret.Name)
			if strings.Contains(secret.Name, "conn") {
				secrets <- secret
			}
		},
	})

	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), secretsInformer.HasSynced)

	err = cli.VanRouterCreate(ctx, types.VanRouterCreateOptions{
		SkupperName:       "skupper",
		IsEdge:            false,
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     false,
		AuthMode:          "",
		User:              "",
		Password:          "",
		ClusterLocal:      true,
	})
	assert.Check(t, err, "Unable to create VAN router")

	for _, c := range testcases {

		err = cli.VanConnectorTokenCreate(ctx, c.connName, testPath+c.connName+".yaml")
		assert.Check(t, err, "Unable to create token")

		err = cli.VanConnectorCreate(ctx, testPath+c.connName+".yaml", types.VanConnectorCreateOptions{
			Name: c.connName,
			Cost: 1,
		})
		assert.Check(t, err, "Unable to create connector")

		select {
		case secret := <-secrets:
			assert.Equal(t, secret.Name, c.secretExpectedName, c.doc)
		case <-time.After(time.Second * 5): //TODO this timeout depend on future running on a real cluster
			//anyway test does not wait for this confition, it is just the error timeout.
			t.Error("Informer did not get the added secret")
		}

	}
	os.RemoveAll(testPath)
}
