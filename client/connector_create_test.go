package client

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func TestConnectorCreateError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("my-namespace", "", "")
	assert.Assert(t, err)

	_, err = cli.ConnectorCreateFromFile(ctx, "./somefile.yaml", types.ConnectorCreateOptions{
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
		opts            []cmp.Option
	}{
		{
			doc:             "Expect generated name to be conn1",
			expectedError:   "",
			connName:        "",
			secretsExpected: []string{"conn1"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect secret name to be as provided: conn2",
			expectedError:   "",
			connName:        "conn2",
			secretsExpected: []string{"conn1", "conn2"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect secret name to be as provided: conn3",
			expectedError:   "",
			connName:        "conn3",
			secretsExpected: []string{"conn1", "conn2", "conn3"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect secret already exists: conn2",
			expectedError:   "A connector secret of that name already exist, please choose a different name",
			connName:        "conn2",
			secretsExpected: []string{"conn1", "conn2", "conn3"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect generated name to be conn4",
			expectedError:   "",
			connName:        "",
			secretsExpected: []string{"conn1", "conn2", "conn3", "conn4"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
	}

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)

	var namespace string = "van-connector-create-interior"

	var cli *VanClient
	var err error
	if *clusterRun {
		cli, err = NewClient(namespace, "", "")
	} else {
		cli, err = newMockClient(namespace, "", "")
	}
	assert.Assert(t, err)

	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

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

	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
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

		err = cli.ConnectorTokenCreateFile(ctx, c.connName, testPath+c.connName+".yaml")
		assert.Assert(t, err, "Unable to create token")

		_, err = cli.ConnectorCreateFromFile(ctx, testPath+c.connName+".yaml", types.ConnectorCreateOptions{
			Name:             c.connName,
			SkupperNamespace: namespace,
			Cost:             1,
		})
		if c.expectedError == "" {
			assert.Assert(t, err, "Unable to create connector")
			// TODO: make more deterministic
			time.Sleep(time.Second * 1)
			if diff := cmp.Diff(c.secretsExpected, secretsFound, c.opts...); diff != "" {
				t.Errorf("TestConnectorCreateInterior "+c.doc+" secrets mismatch (-want +got):\n%s", diff)
			}
			//assert.Assert(t, cmp.Equal(c.secretsExpected, secretsFound, trans), c.doc)
		} else {
			assert.Error(t, err, c.expectedError, c.doc)
			if diff := cmp.Diff(c.secretsExpected, secretsFound, c.opts...); diff != "" {
				t.Errorf("TestConnectorCreateInterior "+c.doc+" secrets mismatch (-want +got):\n%s", diff)
			}
		}

	}
	os.RemoveAll(testPath)
}
