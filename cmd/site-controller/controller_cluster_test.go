package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
)

var siteConfig *corev1.ConfigMap = &corev1.ConfigMap{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "skupper-site",
	},
	Data: map[string]string{
		"name":                   "",
		"edge":                   "false",
		"service-controller":     "true",
		"service-sync":           "true",
		"console":                "true",
		"router-console":         "false",
		"console-authentication": "internal",
		"console-user":           "",
		"console-password":       "",
		"cluster-local":          "true",
	},
}

var connTokenReq *corev1.Secret = &corev1.Secret{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Secret",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "",
		Labels: map[string]string{
			"skupper.io/type": "connection-token-request",
		},
	},
	Data: map[string][]byte{},
}

func waitForConnection(cli *client.VanClient, name string) error {
	var link *types.LinkStatus
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*300)
	defer cancel()

	err = utils.RetryWithContext(ctx, time.Second*5, func() (bool, error) {
		link, err = cli.ConnectorInspect(ctx, name)
		if err != nil {
			return false, nil
		}

		return link.Connected == true, nil
	})

	return err
}

func waitForNoConnections(cli *client.VanClient) error {
	var links []types.LinkStatus
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*300)
	defer cancel()

	err = utils.RetryWithContext(ctx, time.Second*5, func() (bool, error) {
		links, err = cli.ConnectorList(ctx)
		if err != nil {
			return false, nil
		}

		return len(links) == 0, nil
	})

	return err
}

func waitForWorkqueueEmpty(c *SiteController) error {
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*300)
	defer cancel()

	err = utils.RetryWithContext(ctx, time.Second*5, func() (bool, error) {
		return c.workqueue.Len() == 0, nil
	})

	return err
}

func waitForTransportRunning(cli *client.VanClient, namespace string) error {
	pods, err := kube.GetPods("skupper.io/component=router", namespace, cli.KubeClient)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		_, err := kube.WaitForPodStatus(namespace, cli.KubeClient, pod.Name, corev1.PodRunning, time.Second*180, time.Second*5)
		if err != nil {
			return err
		}
	}
	return nil
}

type eventType int

const (
	Add eventType = iota
	Update
	Delete
)

func watchForEvent(cli *client.VanClient, namespace string, name string, event eventType, trigger triggerType, label string, contCh chan<- struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventDoneCh := make(chan struct{})
	stopInformerCh := make(chan struct{})
	defer close(eventDoneCh)
	defer close(stopInformerCh)

	constructor := corev1informer.NewFilteredConfigMapInformer
	if trigger == Token {
		constructor = corev1informer.NewFilteredSecretInformer
	}

	informer := constructor(
		cli.KubeClient,
		namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + name
			options.LabelSelector = label
		}))
	informer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if event == Add {
				eventDoneCh <- struct{}{}
			}
		},
		UpdateFunc: func(old, new interface{}) {
			if event == Update {
				eventDoneCh <- struct{}{}
			}
		},
		DeleteFunc: func(obj interface{}) {
			if event == Delete {
				eventDoneCh <- struct{}{}
			}
		},
	})

	go informer.Run(stopInformerCh)

	if ok := cache.WaitForCacheSync(ctx.Done(), informer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}

	<-eventDoneCh
	contCh <- struct{}{}

	return nil
}

func TestSiteControlleWithCluster(t *testing.T) {
	if !*clusterRun {
		t.Skip("Test to run against real cluster")
	}

	kubeConfigPath := ""
	kubeContext := ""

	// Test:
	// Create sites in two namespaces
	// Create connection token
	// Connect from private to public site
	//

	publicNamespace := "site-controller-cluster-test-" + strings.ToLower(utils.RandomId(4))
	publicCli, err := client.NewClient(publicNamespace, kubeContext, kubeConfigPath)
	assert.Assert(t, err)

	_, err = kube.NewNamespace(publicNamespace, publicCli.KubeClient)
	assert.Check(t, err)
	defer kube.DeleteNamespace(publicNamespace, publicCli.KubeClient)

	privateNamespace := "site-controller-cluster-test-" + strings.ToLower(utils.RandomId(4))
	privateCli, err := client.NewClient(privateNamespace, kubeContext, kubeConfigPath)
	assert.Assert(t, err)

	_, err = kube.NewNamespace(privateNamespace, privateCli.KubeClient)
	assert.Assert(t, err)
	defer kube.DeleteNamespace(privateNamespace, privateCli.KubeClient)

	done := make(chan struct{})
	defer close(done)

	cont := make(chan struct{})
	defer close(cont)

	os.Setenv("WATCH_NAMESPACE", publicNamespace)
	publicController, err := NewSiteController(publicCli)
	assert.Assert(t, err)
	go publicController.Run(done)

	connTokenReq.ObjectMeta.Name = "req1"
	_, err = publicCli.KubeClient.CoreV1().Secrets(publicNamespace).Create(connTokenReq)
	assert.Assert(t, err)

	siteConfig.Data["name"] = publicNamespace
	_, err = publicCli.KubeClient.CoreV1().ConfigMaps(publicNamespace).Create(siteConfig)
	assert.Assert(t, err)

	err = waitForTransportRunning(publicCli, publicNamespace)
	assert.Assert(t, err)

	// Note: looking for update even to change from request to token
	go watchForEvent(publicCli, publicNamespace, "req2", Update, Token, types.TypeTokenQualifier, cont)
	connTokenReq.ObjectMeta.Name = "req2"
	_, err = publicCli.KubeClient.CoreV1().Secrets(publicNamespace).Create(connTokenReq)
	assert.Assert(t, err)
	<-cont

	currentToken, err := publicCli.KubeClient.CoreV1().Secrets(publicNamespace).Get("req2", metav1.GetOptions{})
	assert.Assert(t, err)

	connectSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "connect-2-to-1",
			Labels: map[string]string{
				"skupper.io/type": "connection-token",
			},
			Annotations: currentToken.Annotations,
		},
		Data: currentToken.Data,
	}
	connectSecret.ObjectMeta.Annotations[types.TokenCost] = "5"

	os.Setenv("WATCH_NAMESPACE", privateNamespace)
	privateController, err := NewSiteController(privateCli)
	assert.Assert(t, err)
	go privateController.Run(done)

	siteConfig.Data["name"] = privateNamespace
	_, err = privateCli.KubeClient.CoreV1().ConfigMaps(privateNamespace).Create(siteConfig)
	assert.Assert(t, err)

	err = waitForTransportRunning(privateCli, privateNamespace)
	assert.Assert(t, err)

	// Connect private to public
	go watchForEvent(privateCli, privateNamespace, "connect-2-to-1", Add, Token, types.TypeTokenQualifier, cont)
	_, err = privateCli.KubeClient.CoreV1().Secrets(privateNamespace).Create(connectSecret)
	assert.Assert(t, err)
	<-cont

	// wait for connection
	err = waitForConnection(privateCli, "connect-2-to-1")
	assert.Assert(t, err)

	// get and modify site-config map for one of the namespaces for coverage
	site1, err := publicCli.KubeClient.CoreV1().ConfigMaps(publicNamespace).Get("skupper-site", metav1.GetOptions{})
	assert.Assert(t, err)
	go watchForEvent(publicCli, publicNamespace, "skupper-site", Update, SiteConfig, "", cont)
	site1.ObjectMeta.Annotations = map[string]string{
		"update": "true",
	}
	_, err = publicCli.KubeClient.CoreV1().ConfigMaps(publicNamespace).Update(site1)
	<-cont

	err = privateCli.KubeClient.CoreV1().Secrets(privateNamespace).Delete("connect-2-to-1", &metav1.DeleteOptions{})
	assert.Assert(t, err)

	// check for disconnect
	err = waitForNoConnections(privateCli)
	assert.Assert(t, err)

	// add coverage by letting everything settle out
	err = waitForWorkqueueEmpty(publicController)
	assert.Assert(t, err)

	err = waitForWorkqueueEmpty(privateController)
	assert.Assert(t, err)

	go watchForEvent(publicCli, publicNamespace, "skupper-site", Delete, SiteConfig, "!internal.skupper.io/site-controller-ignore", cont)
	err = publicCli.KubeClient.CoreV1().ConfigMaps(publicNamespace).Delete("skupper-site", &metav1.DeleteOptions{})
	assert.Assert(t, err)
	<-cont

	go watchForEvent(privateCli, privateNamespace, "skupper-site", Delete, SiteConfig, "!internal.skupper.io/site-controller-ignore", cont)
	err = privateCli.KubeClient.CoreV1().ConfigMaps(privateNamespace).Delete("skupper-site", &metav1.DeleteOptions{})
	assert.Assert(t, err)
	<-cont

}
