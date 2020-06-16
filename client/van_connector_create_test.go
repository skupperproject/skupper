package client

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func TestConnectorCreateError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("my-namespace", "", "")
	assert.Assert(t, err)

	_, err = cli.VanConnectorCreateFromFile(ctx, "./somefile.yaml", types.VanConnectorCreateOptions{
		Name: "",
		Cost: 1,
	})
	assert.Error(t, err, "open ./somefile.yaml: no such file or directory", "Expect error when file not found")
}

func TestConnectorCreateInterior(t *testing.T) {
	testcases := []struct {
		doc             string
		expectedError   string
		connName        string
		connFile        string
		secretsExpected []string
	}{
		{
			doc:             "Expect generated name to be conn1",
			expectedError:   "",
			connName:        "",
			secretsExpected: []string{"conn1"},
		},
		{
			doc:             "Expect secret name to be as provided: conn2",
			expectedError:   "",
			connName:        "conn2",
			secretsExpected: []string{"conn2"},
		},
	}

	trans := cmp.Transformer("Sort", func(in []string) []string {
		out := append([]string(nil), in...)
		sort.Strings(out)
		return out
	})

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)

	var namespace string = "testconnectorcreateinterior"

	var cli *VanClient
	var err error
	if *clusterRun {
		cli, err = NewClient(namespace, "", "")
	} else {
		cli, err = newMockClient(namespace, "", "")
	}
	assert.Assert(t, err)

	createNamespace := func() {
		NsSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		_, err := cli.KubeClient.CoreV1().Namespaces().Create(NsSpec)
		assert.Assert(t, err)
	}

	deleteNamespace := func() {
		cli.KubeClient.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	}

	createNamespace()
	defer deleteNamespace()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secretsFound := []string{}
	informers := informers.NewSharedInformerFactory(cli.KubeClient, 0)
	secretsInformer := informers.Core().V1().Secrets().Informer()
	secretsInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*corev1.Secret)
			if !strings.HasPrefix(secret.Name, "skupper") {
				secretsFound = append(secretsFound, secret.Name)
			}
		},
	})

	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), secretsInformer.HasSynced)

	err = cli.VanRouterCreate(ctx, types.VanSiteConfig{
		Spec: types.VanSiteConfigSpec{
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
	assert.Assert(t, err, "Unable to create VAN router")

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		secretsFound = []string{}

		err = cli.VanConnectorTokenCreateFile(ctx, c.connName, testPath+c.connName+".yaml")
		assert.Assert(t, err, "Unable to create token")

		_, err = cli.VanConnectorCreateFromFile(ctx, testPath+c.connName+".yaml", types.VanConnectorCreateOptions{
			Name: c.connName,
			Cost: 1,
		})
		assert.Assert(t, err, "Unable to create connector")

		// TODO: make more deterministic
		time.Sleep(time.Second * 1)
		assert.Assert(t, cmp.Equal(c.secretsExpected, secretsFound, trans), c.doc)
	}

	os.RemoveAll(testPath)
}
