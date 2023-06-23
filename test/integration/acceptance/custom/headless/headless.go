package headless

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/test/utils/tools"
	apiv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"gotest.tools/assert"
)

const (
	appName     = "nginx"
	appPort     = "8080"
	appPortInt  = 8080
	appPortName = "web"
	appImage    = "quay.io/dhashimo/nginx-unprivileged:stable-alpine"
	pod0Name    = appName + "-0"
	pod1Name    = appName + "-1"
)

type BasicTestRunner struct {
	base.ClusterTestRunnerBase
}

func (r *BasicTestRunner) TestsHeadlessElements(ctx context.Context, t *testing.T) {

	const (
		pod0ProxyName = appName + "-proxy-0"
		pod1ProxyName = appName + "-proxy-1"
		svcProxyName  = appName + "-proxy"
		timeout       = 120 * time.Second
		interval      = 5 * time.Second
	)

	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, pubCluster, 1))
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, prvCluster, 1))

	t.Run("headless-inspect-pods", func(t *testing.T) {
		t.Logf("Testing: %s\n", "All proxy pods got deployed")

		// Check proxy pods in private - created by skupper
		_, err = kube.WaitForPodStatus(prvCluster.Namespace, prvCluster.VanClient.KubeClient, pod0ProxyName, corev1.PodRunning, timeout, interval)
		assert.Assert(t, err)
		_, err = kube.WaitForPodStatus(prvCluster.Namespace, prvCluster.VanClient.KubeClient, pod1ProxyName, corev1.PodRunning, timeout, interval)
		assert.Assert(t, err)

		// Check app pods in public - created by skupper
		_, err = kube.WaitForPodStatus(pubCluster.Namespace, pubCluster.VanClient.KubeClient, pod0Name, corev1.PodRunning, timeout, interval)
		assert.Assert(t, err)
		_, err = kube.WaitForPodStatus(pubCluster.Namespace, pubCluster.VanClient.KubeClient, pod1Name, corev1.PodRunning, timeout, interval)
		assert.Assert(t, err)
	})

	t.Run("headless-inspect-svcs", func(t *testing.T) {
		t.Logf("Testing: %s\n", "All proxy services got deployed")

		// Check if services exists in both sides
		_, err = prvCluster.VanClient.KubeClient.CoreV1().Services(prvCluster.Namespace).Get(context.TODO(), svcProxyName, metav1.GetOptions{})
		assert.Assert(t, err)
		_, err = pubCluster.VanClient.KubeClient.CoreV1().Services(pubCluster.Namespace).Get(context.TODO(), appName, metav1.GetOptions{})
		assert.Assert(t, err)
	})

	t.Run("headless-inspect-svc-name-unique", func(t *testing.T) {
		t.Logf("Testing: %s\n", "All proxy services use unique service names")
		stsSvc, err := prvCluster.VanClient.KubeClient.AppsV1().StatefulSets(prvCluster.Namespace).Get(context.TODO(), appName, metav1.GetOptions{})
		assert.Assert(t, err)
		stsSvcProxy, err := prvCluster.VanClient.KubeClient.AppsV1().StatefulSets(prvCluster.Namespace).Get(context.TODO(), svcProxyName, metav1.GetOptions{})
		assert.Assert(t, err)
		assert.Assert(t, stsSvc.Spec.ServiceName != stsSvcProxy.Spec.ServiceName)
	})

	t.Run("headless-inspect-svc-selector-unchanged", func(t *testing.T) {
		t.Logf("Testing: %s\n", "Original service name kept unchanged")
		prvSvc, err := prvCluster.VanClient.KubeClient.CoreV1().Services(prvCluster.Namespace).Get(context.TODO(), appName, metav1.GetOptions{})
		assert.Assert(t, err)
		assert.Assert(t, prvSvc.Spec.Selector["app"] == appName)
	})

	t.Run("headless-inspect-routerid-unique", func(t *testing.T) {
		t.Logf("Testing: %s\n", "Routers have unique ids")
		skpIntConfigMap, err := pubCluster.VanClient.KubeClient.CoreV1().ConfigMaps(pubCluster.Namespace).Get(context.TODO(), "skupper-site", metav1.GetOptions{})
		assert.Assert(t, err)
		skpSiteID := skpIntConfigMap.ObjectMeta.UID

		stsEnvs, err := pubCluster.KubectlExec("get statefulset.apps/" + appName + " -o jsonpath=\"{.spec.template.spec.containers[0].env[0].value}\"")
		assert.Assert(t, err)
		assert.Assert(t, strings.Contains(string(stsEnvs), "\"id\": \"${HOSTNAME}-"+string(skpSiteID)+"\","), "SkupperID not found as part of the routerID in proxy spec")
	})
}

func testHTTPAccess(t *testing.T, cluster *base.ClusterContext, accessURL string, expectedInAnswer string) {

	var CurlOptsForTest tools.CurlOpts
	CurlOptsForTest = tools.CurlOpts{Silent: true, Insecure: true, Timeout: 10, Verbose: true}

	res, err := tools.Curl(cluster.VanClient.KubeClient, cluster.VanClient.RestConfig, cluster.Namespace, "", accessURL, CurlOptsForTest)
	assert.Assert(t, err)
	assert.Assert(t, strings.Contains(res.Output, expectedInAnswer), "Expected value %s not found in test answer", expectedInAnswer)
}

func (r *BasicTestRunner) TestAccessToPods(ctx context.Context, t *testing.T) {

	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, pubCluster, 1))
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, prvCluster, 1))

	podZeroPub, err := pubCluster.VanClient.KubeClient.CoreV1().Pods(pubCluster.Namespace).Get(context.TODO(), pod0Name, metav1.GetOptions{})
	assert.Assert(t, err)
	podOnePub, err := pubCluster.VanClient.KubeClient.CoreV1().Pods(pubCluster.Namespace).Get(context.TODO(), pod1Name, metav1.GetOptions{})
	assert.Assert(t, err)
	podZeroPubIP := podZeroPub.Status.PodIP
	podOnePubIP := podOnePub.Status.PodIP

	podOnePriv, err := pubCluster.VanClient.KubeClient.CoreV1().Pods(prvCluster.Namespace).Get(context.TODO(), pod0Name, metav1.GetOptions{})
	assert.Assert(t, err)
	podTwoPriv, err := pubCluster.VanClient.KubeClient.CoreV1().Pods(prvCluster.Namespace).Get(context.TODO(), pod1Name, metav1.GetOptions{})
	assert.Assert(t, err)
	podZeroPrivIP := podOnePriv.Status.PodIP
	podOnePrivIP := podTwoPriv.Status.PodIP

	testCases := []struct {
		id      string
		doc     string
		expect  string
		url     string
		cluster *base.ClusterContext
	}{
		{
			id:      "headless-access-pod0-private",
			doc:     "Access application by its static address (pod0) in private cluster",
			expect:  fmt.Sprintf("%s.%s (%s)", pod0Name, appName, podZeroPrivIP),
			url:     fmt.Sprintf("http://%s.%s:%s", pod0Name, appName, appPort),
			cluster: prvCluster,
		},
		{
			id:      "headless-access-pod1-private",
			doc:     "Access application by its static address (pod1) in private cluster",
			expect:  fmt.Sprintf("%s.%s (%s)", pod1Name, appName, podOnePrivIP),
			url:     fmt.Sprintf("http://%s.%s:%s", pod1Name, appName, appPort),
			cluster: prvCluster,
		},
		{
			id:      "headless-access-pod0-public",
			doc:     "Access application by its static address (pod0) in public cluster",
			expect:  fmt.Sprintf("%s.%s (%s)", pod0Name, appName, podZeroPubIP),
			url:     fmt.Sprintf("http://%s.%s:%s", pod0Name, appName, appPort),
			cluster: pubCluster,
		},
		{
			id:      "headless-access-pod1-public",
			doc:     "Access application by its static address (pod1) in public cluster",
			expect:  fmt.Sprintf("%s.%s (%s)", pod1Name, appName, podOnePubIP),
			url:     fmt.Sprintf("http://%s.%s:%s", pod1Name, appName, appPort),
			cluster: pubCluster,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.id, func(t *testing.T) {
			t.Logf("Testing: %s\n", testCase.doc)
			testHTTPAccess(t, testCase.cluster, testCase.url, testCase.expect)
		})
	}
}

func createHeadlessStatefulSet(cluster *client.VanClient, annotations map[string]string) (*apiv1.StatefulSet, error) {

	name := appName
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{Name: appPortName, Port: appPortInt},
			},
			Selector: map[string]string{
				"app": name,
			},
			PublishNotReadyAddresses: true,
		},
	}

	// Creating the new service
	svc, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	replicas := int32(2)
	ss := &apiv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cluster.Namespace,
			Annotations: annotations,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: apiv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: appName, Image: appImage, Ports: []corev1.ContainerPort{{Name: appPortName, ContainerPort: appPortInt}}, ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	ss, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Create(context.TODO(), ss, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Wait for statefulSet to be ready
	ss, err = kube.WaitStatefulSetReadyReplicas(ss.Name, cluster.Namespace, 2, cluster.KubeClient, 120*time.Second, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return ss, nil
}

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.SiteConfigSpec, createOptsPrivate types.SiteConfigSpec, tokenType string, t *testing.T) {

	testContext, cancel := context.WithTimeout(ctx, types.DefaultTimeoutDuration*2)
	defer cancel()

	var err error
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	// Headless annotations
	headlessStatefulSetAnnotations := map[string]string{}
	headlessStatefulSetAnnotations[types.ProxyQualifier] = "tcp"
	headlessStatefulSetAnnotations[types.AddressQualifier] = appName
	headlessStatefulSetAnnotations[types.HeadlessQualifier] = "true"

	_, err = createHeadlessStatefulSet(prv1Cluster.VanClient, headlessStatefulSetAnnotations)
	assert.Assert(t, err)

	createOptsPublic.SkupperNamespace = pub1Cluster.Namespace
	siteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPublic)
	assert.Assert(t, err)
	err = pub1Cluster.VanClient.RouterCreate(testContext, *siteConfig)
	assert.Assert(t, err)
	err = base.WaitSkupperRunning(pub1Cluster)
	assert.Assert(t, err)

	const secretFile = "/tmp/public_headless_1_secret.yaml"
	if tokenType == "claim" {
		err = pub1Cluster.VanClient.TokenClaimCreateFile(ctx, types.DefaultVanName, []byte(createOptsPublic.Password), 15*time.Minute, 1, secretFile)
	} else {
		err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	}
	assert.Assert(t, err)

	createOptsPrivate.SkupperNamespace = prv1Cluster.Namespace
	siteConfig, err = prv1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPrivate)
	assert.Assert(t, err)
	err = prv1Cluster.VanClient.RouterCreate(testContext, *siteConfig)
	assert.Assert(t, err)
	err = base.WaitSkupperRunning(prv1Cluster)
	assert.Assert(t, err)

	var connectorCreateOpts = types.ConnectorCreateOptions{
		SkupperNamespace: prv1Cluster.Namespace,
		Name:             "",
		Cost:             0,
	}
	_, err = prv1Cluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	assert.Assert(t, err)
}

func (r *BasicTestRunner) TearDown(ctx context.Context) {
	errMsg := "Something failed! aborting teardown"

	pub, err := r.GetPublicContext(1)
	if err != nil {
		log.Print(errMsg)
		return
	}

	priv, err := r.GetPrivateContext(1)
	if err != nil {
		log.Print(errMsg)
		return
	}

	_ = pub.DeleteNamespace()
	_ = priv.DeleteNamespace()
}

func (r *BasicTestRunner) Run(ctx context.Context, t *testing.T) {

	defer r.TearDown(ctx)
	defer r.DumpOnFailure(t)
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)
	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)
	err = pub1Cluster.CreateNamespace()
	assert.Assert(t, err)
	err = prv1Cluster.CreateNamespace()
	assert.Assert(t, err)

	createOptsPublic := types.SiteConfigSpec{
		SkupperName:       "",
		RouterMode:        string(types.TransportModeInterior),
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     false,
		Ingress:           pub1Cluster.VanClient.GetIngressDefault(),
		Replicas:          1,
		Router:            constants.DefaultRouterOptions(nil),
	}

	createOptsPrivate := types.SiteConfigSpec{
		SkupperName:       "",
		RouterMode:        string(types.TransportModeInterior),
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     false,
		Ingress:           prv1Cluster.VanClient.GetIngressDefault(),
		Replicas:          1,
		Router:            constants.DefaultRouterOptions(nil),
	}

	// Unique setup and teardown - Run testcases in the same scenario
	r.Setup(ctx, createOptsPublic, createOptsPrivate, "", t)

	// Expose and inspect elements
	testID := "headless-inspect-elements"
	testDoc := "Expose a headless service and inspect its elements"
	t.Run(testID, func(t *testing.T) {
		t.Logf("Testing: %s\n", testDoc)
		r.TestsHeadlessElements(ctx, t)
	})

	// Access pods via name
	testID = "headless-access-pod-name"
	testDoc = "Access an application inside a specific pod by its name"
	t.Run(testID, func(t *testing.T) {
		t.Logf("Testing: %s\n", testDoc)
		r.TestAccessToPods(ctx, t)
	})
}

func (r *BasicTestRunner) DumpOnFailure(t *testing.T) {
	if t.Failed() {
		r.DumpTestInfo(r.Needs.NamespaceId)
	}
}
