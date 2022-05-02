package hipstershop

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RESOURCES_PRV1 = "https://raw.githubusercontent.com/skupperproject/skupper-example-grpc/master/deployment-ms-a.yaml"
	RESOURCES_PUB1 = "https://raw.githubusercontent.com/skupperproject/skupper-example-grpc/master/deployment-ms-b.yaml"
	RESOURCES_PUB2 = "https://raw.githubusercontent.com/skupperproject/skupper-example-grpc/master/deployment-ms-c.yaml"
)

var (
	ctx, cancelFn    = context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	cleanUpDirs      []string
	exposed_services = []string{
		"productcatalogservice",
		"recommendationservice",
		"checkoutservice",
		"cartservice",
		"currencyservice",
		"adservice",
		"emailservice",
		"paymentservice",
		"shippingservice",
	}
)

func Setup(t *testing.T, testRunner base.ClusterTestRunner) {
	var err error

	pub1, err := testRunner.GetPublicContext(1)
	assert.Assert(t, err)
	pub2, err := testRunner.GetPublicContext(2)
	assert.Assert(t, err)
	prv1, err := testRunner.GetPrivateContext(1)
	assert.Assert(t, err)

	// creating namespaces
	assert.Assert(t, pub1.CreateNamespace())
	assert.Assert(t, pub2.CreateNamespace())
	assert.Assert(t, prv1.CreateNamespace())
}

func CreateVAN(t *testing.T, testRunner base.ClusterTestRunner) {
	var err error
	t.Logf("Creating VANs")
	pub1, _ := testRunner.GetPublicContext(1)
	pub2, _ := testRunner.GetPublicContext(2)
	prv1, _ := testRunner.GetPrivateContext(1)

	siteConfigSpec := types.SiteConfigSpec{
		SiteControlled:    true,
		EnableController:  true,
		EnableServiceSync: true,
		User:              "admin",
		Password:          "admin",
		Router:            constants.DefaultRouterOptions(nil),
	}

	// If using only 1 cluster, set ClusterLocal to True
	if !base.MultipleClusters() {
		siteConfigSpec.Ingress = "none"
	}

	// Creating the router on public1 cluster
	siteConfig, err := pub1.VanClient.SiteConfigCreate(ctx, siteConfigSpec)
	assert.Assert(t, err, "error creating site config for public1 cluster")
	err = pub1.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err, "error creating router on public1 cluster")
	t.Logf("setting siteConfigSpec to run with Ingress=%v", siteConfig.Spec.Ingress)

	// Creating the router on public2 cluster
	siteConfig, err = pub2.VanClient.SiteConfigCreate(ctx, siteConfigSpec)
	assert.Assert(t, err, "error creating site config for public2 cluster")
	err = pub2.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err, "error creating router on public2 cluster")

	// Creating the router on private1 cluster
	siteConfig, err = prv1.VanClient.SiteConfigCreate(ctx, siteConfigSpec)
	assert.Assert(t, err, "error creating site config for private1 cluster")
	err = prv1.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err, "error creating router on private1 cluster")

	// Creating a local directory for storing the token
	tokenPath, err := ioutil.TempDir("", "token")
	assert.Assert(t, err, "error creating temporary directory for tokens")
	cleanUpDirs = append(cleanUpDirs, tokenPath)

	// Creating token and connecting sites
	tokenPub1 := tokenPath + "/public1.yaml"
	err = pub1.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, tokenPub1)
	assert.Assert(t, err, "unable to create token to public1 cluster")

	tokenPub2 := tokenPath + "/public2.yaml"
	err = pub2.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, tokenPub2)
	assert.Assert(t, err, "unable to create token to public2 cluster")

	// Connecting public2 to public1
	secret, err := pub2.VanClient.ConnectorCreateFromFile(ctx, tokenPub1, types.ConnectorCreateOptions{SkupperNamespace: pub2.Namespace})
	assert.Assert(t, err, "unable to create connection from public2 to public1 cluster")
	assert.Assert(t, secret != nil)

	// Connecting private1 to public1
	secret, err = prv1.VanClient.ConnectorCreateFromFile(ctx, tokenPub1, types.ConnectorCreateOptions{SkupperNamespace: prv1.Namespace})
	assert.Assert(t, err, "unable to create connection from private1 to public1 cluster")
	assert.Assert(t, secret != nil)

	// Connecting private1 to public2
	secret, err = prv1.VanClient.ConnectorCreateFromFile(ctx, tokenPub2, types.ConnectorCreateOptions{SkupperNamespace: prv1.Namespace})
	assert.Assert(t, err, "unable to create connection from private1 to public2 cluster")
	assert.Assert(t, secret != nil)

	// Waiting till Skupper is running and all clusters are connected
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, pub1, 2))
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, pub2, 2))
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, prv1, 2))
}

func DeployResources(t *testing.T, testRunner base.ClusterTestRunner) {
	prv1, _ := testRunner.GetPrivateContext(1)
	pub1, _ := testRunner.GetPublicContext(1)
	pub2, _ := testRunner.GetPublicContext(2)

	prv1AppLabels := []string{"frontend", "productcatalogservice", "recommendationservice"}
	pub1AppLabels := []string{"adservice", "cartservice", "checkoutservice", "currencyservice", "redis-cart"}
	pub2AppLabels := []string{"emailservice", "paymentservice", "shippingservice"}

	t.Logf("Deploying microservices on %s", prv1.Namespace)
	assert.Assert(t, k8s.CreateResourcesFromYAML(prv1.VanClient, RESOURCES_PRV1))
	t.Logf("Deploying microservices on %s", pub1.Namespace)
	assert.Assert(t, k8s.CreateResourcesFromYAML(pub1.VanClient, RESOURCES_PUB1))
	t.Logf("Deploying microservices on %s", pub2.Namespace)
	assert.Assert(t, k8s.CreateResourcesFromYAML(pub2.VanClient, RESOURCES_PUB2))

	// Wait till all pods for the given labels are running or till it times out
	waitPodsRunning := func(cluster *base.ClusterContext, appLabels []string) error {
		t.Logf("Waiting on microservices to be running at %s", cluster.Namespace)
		err := utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
			// Waiting on pods to be available
			podList, err := cluster.VanClient.KubeClient.CoreV1().Pods(cluster.Namespace).List(v1.ListOptions{
				LabelSelector: fmt.Sprintf("app in (%s)", strings.Join(appLabels, ",")),
			})
			if err != nil {
				return true, err
			}
			if len(podList.Items) != len(appLabels) {
				return false, nil
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != v12.PodRunning {
					return false, nil
				}
			}
			return true, nil
		})
		t.Logf("Pod status at %s:", cluster.Namespace)
		cluster.KubectlExec("get pods")
		// If an error has occurred, verify the events as well
		if err != nil {
			t.Logf("Latest events")
			cluster.KubectlExec("get events")
		}
		return err
	}
	assert.Assert(t, waitPodsRunning(prv1, prv1AppLabels))
	assert.Assert(t, waitPodsRunning(pub1, pub1AppLabels))
	assert.Assert(t, waitPodsRunning(pub2, pub2AppLabels))
}

func ExposeResources(t *testing.T, testRunner base.ClusterTestRunner) {
	t.Logf("Exposing services through Skupper")
	prv1, _ := testRunner.GetPrivateContext(1)
	pub1, _ := testRunner.GetPublicContext(1)
	pub2, _ := testRunner.GetPublicContext(2)

	// Exposing resources
	exposePrivate1Resources(t, prv1)
	exposePublic1Resources(t, pub1)
	exposePublic2Resources(t, pub2)

	// Wait till all services are available across all clusters/namespaces
	for _, cluster := range []*base.ClusterContext{prv1, pub1, pub2} {
		for _, svc := range exposed_services {
			t.Logf("Waiting on %s to be available on %s", svc, cluster.Namespace)
			_, err := k8s.WaitForServiceToBeAvailableDefaultTimeout(cluster.Namespace, cluster.VanClient.KubeClient, svc)
			assert.Assert(t, err)
		}
	}
}

func exposePrivate1Resources(t *testing.T, prv1 *base.ClusterContext) {
	// Exposing resources from private1 cluster
	productCatalogSvc := &types.ServiceInterface{
		Address:  "productcatalogservice",
		Protocol: "http2",
		Ports:    []int{3550},
	}
	recommendationSvc := &types.ServiceInterface{
		Address:  "recommendationservice",
		Protocol: "http2",
		Ports:    []int{8080},
	}
	// Creating services
	t.Logf("Creating service interfaces in private1 cluster")
	assert.Assert(t, prv1.VanClient.ServiceInterfaceCreate(ctx, productCatalogSvc))
	assert.Assert(t, prv1.VanClient.ServiceInterfaceCreate(ctx, recommendationSvc))
	t.Logf("Binding service interfaces in private1 cluster")
	assert.Assert(t, prv1.VanClient.ServiceInterfaceBind(ctx, productCatalogSvc, "deployment", productCatalogSvc.Address, "http2", map[int]int{3550: 3550}))
	assert.Assert(t, prv1.VanClient.ServiceInterfaceBind(ctx, recommendationSvc, "deployment", recommendationSvc.Address, "http2", map[int]int{8080: 8080}))
}

func exposePublic1Resources(t *testing.T, pub1 *base.ClusterContext) {
	// Exposing resources from public1 cluster
	checkoutSvc := &types.ServiceInterface{
		Address:  "checkoutservice",
		Protocol: "http2",
		Ports:    []int{5050},
	}
	cartSvc := &types.ServiceInterface{
		Address:  "cartservice",
		Protocol: "http2",
		Ports:    []int{7070},
	}
	currencySvc := &types.ServiceInterface{
		Address:  "currencyservice",
		Protocol: "http2",
		Ports:    []int{7000},
	}
	adSvc := &types.ServiceInterface{
		Address:  "adservice",
		Protocol: "http2",
		Ports:    []int{9555},
	}
	redisSvc := &types.ServiceInterface{
		Address:  "redis-cart",
		Protocol: "tcp",
		Ports:    []int{6379},
	}
	// Creating services
	t.Logf("creating service interfaces in public1 cluster")
	assert.Assert(t, pub1.VanClient.ServiceInterfaceCreate(ctx, checkoutSvc))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceCreate(ctx, cartSvc))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceCreate(ctx, currencySvc))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceCreate(ctx, adSvc))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceCreate(ctx, redisSvc))
	t.Logf("binding service interfaces in public1 cluster")
	assert.Assert(t, pub1.VanClient.ServiceInterfaceBind(ctx, checkoutSvc, "deployment", checkoutSvc.Address, "http2", map[int]int{5050: 5050}))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceBind(ctx, cartSvc, "deployment", cartSvc.Address, "http2", map[int]int{7070: 7070}))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceBind(ctx, currencySvc, "deployment", currencySvc.Address, "http2", map[int]int{7000: 7000}))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceBind(ctx, adSvc, "deployment", adSvc.Address, "http2", map[int]int{9555: 9555}))
	assert.Assert(t, pub1.VanClient.ServiceInterfaceBind(ctx, redisSvc, "deployment", redisSvc.Address, "tcp", map[int]int{6379: 6379}))
}

func exposePublic2Resources(t *testing.T, pub2 *base.ClusterContext) {
	// Exposing resources from public2 cluster
	paymentSvc := &types.ServiceInterface{
		Address:  "paymentservice",
		Protocol: "http2",
		Ports:    []int{50051},
	}
	shippingSvc := &types.ServiceInterface{
		Address:  "shippingservice",
		Protocol: "http2",
		Ports:    []int{50051},
	}
	emailSvc := &types.ServiceInterface{
		Address:  "emailservice",
		Protocol: "http2",
		Ports:    []int{5000},
	}
	// Creating services
	t.Logf("creating service interfaces in public2 cluster")
	assert.Assert(t, pub2.VanClient.ServiceInterfaceCreate(ctx, paymentSvc))
	assert.Assert(t, pub2.VanClient.ServiceInterfaceCreate(ctx, shippingSvc))
	assert.Assert(t, pub2.VanClient.ServiceInterfaceCreate(ctx, emailSvc))
	t.Logf("binding service interfaces in public2 cluster")
	assert.Assert(t, pub2.VanClient.ServiceInterfaceBind(ctx, paymentSvc, "deployment", paymentSvc.Address, "http2", map[int]int{50051: 50051}))
	assert.Assert(t, pub2.VanClient.ServiceInterfaceBind(ctx, shippingSvc, "deployment", shippingSvc.Address, "http2", map[int]int{50051: 50051}))
	assert.Assert(t, pub2.VanClient.ServiceInterfaceBind(ctx, emailSvc, "deployment", emailSvc.Address, "http2", map[int]int{5000: 8080}))
}
