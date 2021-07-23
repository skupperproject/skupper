package annotation

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"gotest.tools/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	timeout  = 120 * time.Second
	interval = 20 * time.Second
)

var (
	ctx, cancelFn = context.WithTimeout(context.Background(), constants.TestSuiteTimeout)
)

// test allows defining the matrix to run the test table
type test struct {
	name                  string
	doc                   string
	modification          func(*testing.T, base.ClusterTestRunner)
	expectedSites         int
	expectedServicesProto map[string]string
}

// Setup simply creates the namespaces using a single
// cluster by default, or using two clusters if at
// least two kubeconfig files have been provided. It
// also initializes the cluster1 and cluster2 variables.
func Setup(t *testing.T, testRunner base.ClusterTestRunner) {
	var err error

	pub, err := testRunner.GetPublicContext(1)
	assert.Assert(t, err)
	prv, err := testRunner.GetPrivateContext(1)
	assert.Assert(t, err)

	// creating namespaces
	assert.Assert(t, pub.CreateNamespace())
	assert.Assert(t, prv.CreateNamespace())
}

// Teardown ensures all resources are removed
// when test completes
func TearDown(t *testing.T, testRunner base.ClusterTestRunner) {
	t.Logf("Tearing down")
	t.Log("Deleting namespaces")
	base.TearDownSimplePublicAndPrivate(testRunner.(*base.ClusterTestRunnerBase))
}

// DeployResources provides common setup for kicking off all annotated
// resource tests. It deploys a static set of resources:
// - Deployments
// - Services.
//
// And it also includes static annotations to the deployed resources.
//
// The default resources and annotations (if true) that will
// be added to both cluster1 and cluster2 are:
// deployment/nginx  ## cluster1
//   annotations:
//     skupper.io/proxy: tcp
//     skupper.io/address: nginx-1-dep-web
// statefulset/nginx  ## cluster1
//   annotations:
//     skupper.io/proxy: tcp
//     skupper.io/address: nginx-1-ss-web
// daemonset/nginx  ## cluster1
//   annotations:
//     skupper.io/proxy: tcp
//     skupper.io/address: nginx-1-ds-web
// service/nginx-1-svc-exp-notarget  ## cluster1
//   annotations:
//     skupper.io/proxy: tcp
// service/nginx-1-svc-target  ## cluster1
//   annotations:
//     skupper.io/proxy: http
//     skupper.io/address: nginx-1-svc-exp-target
// deployment/nginx  ## cluster2
//   annotations:
//     skupper.io/proxy: tcp
//     skupper.io/address: nginx-2-dep-web
// statefulset/nginx  ## cluster2
//   annotations:
//     skupper.io/proxy: tcp
//     skupper.io/address: nginx-2-ss-web
// daemonset/nginx  ## cluster2
//   annotations:
//     skupper.io/proxy: tcp
//     skupper.io/address: nginx-2-ds-web
// service/nginx-2-svc-exp-notarget  ## cluster2
//   annotations:
//     skupper.io/proxy: tcp
// service/nginx-2-svc-target  ## cluster2
//   annotations:
//     skupper.io/proxy: http
//     skupper.io/address: nginx-1-svc-exp-target
//
func DeployResources(t *testing.T, testRunner base.ClusterTestRunner) {
	// Deploys a static set of resources
	t.Logf("Deploying resources")

	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	// Deploys the same set of resources against both clusters
	// resources will have index (1 or 2), depending on the
	// cluster they are being deployed to
	for i, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		clusterIdx := i + 1

		// Annotations (optional) to deployment and services
		depAnnotations := map[string]string{}
		statefulSetAnnotations := map[string]string{}
		daemonSetAnnotations := map[string]string{}
		svcNoTargetAnnotations := map[string]string{}
		svcTargetAnnotations := map[string]string{}
		populateAnnotations(clusterIdx, depAnnotations, svcNoTargetAnnotations, svcTargetAnnotations,
			statefulSetAnnotations, daemonSetAnnotations)

		// One single deployment will be created (for the nginx http server)
		createDeployment(t, cluster, depAnnotations)
		createStatefulSet(t, cluster, statefulSetAnnotations)
		createDaemonSet(t, cluster, daemonSetAnnotations)

		// Now create two services. One that does not have a target address,
		// and another that provides a target address.
		createService(t, cluster, fmt.Sprintf("nginx-%d-svc-exp-notarget", clusterIdx), svcNoTargetAnnotations)
		// This service with the target should not be exposed (only the target service will be)
		createService(t, cluster, fmt.Sprintf("nginx-%d-svc-target", clusterIdx), svcTargetAnnotations)
	}

	// Wait for pods to be running
	for _, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		t.Logf("waiting on pods to be running on %s", cluster.Namespace)
		// Get all pod names
		podList, err := cluster.KubeClient.CoreV1().Pods(cluster.Namespace).List(metav1.ListOptions{})
		assert.Assert(t, err)
		assert.Assert(t, len(podList.Items) > 0)

		for _, pod := range podList.Items {

			_, err := kube.WaitForPodStatus(cluster.Namespace, cluster.KubeClient, pod.Name, corev1.PodRunning, timeout, interval)
			assert.Assert(t, err)
		}
	}
}

// populateAnnotations annotates the provide maps with static
// annotations for the deployment, and for each of the services
func populateAnnotations(clusterIdx int, depAnnotations map[string]string, svcNoTargetAnnotations map[string]string, svcTargetAnnotations map[string]string,
	statefulSetAnnotations map[string]string, daemonSetAnnotations map[string]string) {
	// Define a static set of annotations to the deployment
	depAnnotations[types.ProxyQualifier] = "tcp"
	depAnnotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-dep-web", clusterIdx)

	// Set annotations to the service with no target address
	svcNoTargetAnnotations[types.ProxyQualifier] = "tcp"

	// Set annotations to the service with target address
	svcTargetAnnotations[types.ProxyQualifier] = "http"
	svcTargetAnnotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-svc-exp-target", clusterIdx)

	//set annotations on statefulset
	statefulSetAnnotations[types.ProxyQualifier] = "tcp"
	statefulSetAnnotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-ss-web", clusterIdx)

	//set annotations on daemonset
	daemonSetAnnotations[types.ProxyQualifier] = "tcp"
	daemonSetAnnotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-ds-web", clusterIdx)
}

// CreateVan deploys Skupper to both clusters then generates a
// connection token on cluster1 and connects cluster2 to cluster1
// waiting for the network to be formed (or failing otherwise)
func CreateVan(t *testing.T, testRunner base.ClusterTestRunner) {
	var err error
	t.Logf("Creating VANs")
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	siteConfigSpec := types.SiteConfigSpec{
		SiteControlled:    true,
		EnableController:  true,
		EnableServiceSync: true,
		User:              "admin",
		Password:          "admin",
		Ingress:           pub.VanClient.GetIngressDefault(),
	}

	// If using only 1 cluster, set ClusterLocal to True
	if !base.MultipleClusters() {
		siteConfigSpec.Ingress = types.IngressNoneString
	}
	t.Logf("setting siteConfigSpec to run with Ingress=%v", siteConfigSpec.Ingress)

	// Creating the router on cluster1
	siteConfig, err := pub.VanClient.SiteConfigCreate(ctx, siteConfigSpec)
	assert.Assert(t, err, "error creating site config for public cluster")
	err = pub.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err, "error creating router on cluster1")

	// Creating the router on cluster2
	siteConfig, err = prv.VanClient.SiteConfigCreate(ctx, siteConfigSpec)
	assert.Assert(t, err, "error creating site config for private cluster")
	err = prv.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err, "error creating router on cluster2")

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)

	// Creating token and connecting sites
	tokenFile := testPath + "cluster1.yaml"
	err = pub.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, tokenFile)
	assert.Assert(t, err, "unable to create token to cluster1")

	// Connecting cluster2 to cluster1
	secret, err := prv.VanClient.ConnectorCreateFromFile(ctx, tokenFile, types.ConnectorCreateOptions{SkupperNamespace: prv.Namespace})
	assert.Assert(t, err, "unable to create connection to cluster1")
	assert.Assert(t, secret != nil)

	// Waiting till skupper is running on both clusters
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, pub, 1))
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, prv, 1))
}

// createDeployment creates a pre-defined nginx Deployment at the given
// cluster and namespace. Reason for using it is that it is a tiny image
// and allows tests to validate traffic flowing using both http and tcp bridges.
func createDeployment(t *testing.T, cluster *client.VanClient, annotations map[string]string) *v1.Deployment {
	name := "nginx"
	replicas := int32(1)
	dep := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cluster.Namespace,
			Annotations: annotations,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
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
						{Name: "nginx", Image: "nginxinc/nginx-unprivileged:stable-alpine", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 8080}}, ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Create(dep)
	assert.Assert(t, err)

	// Wait for deployment to be ready
	dep, err = kube.WaitDeploymentReadyReplicas(dep.Name, cluster.Namespace, 1, cluster.KubeClient, timeout, interval)
	assert.Assert(t, err)

	return dep
}

func createStatefulSet(t *testing.T, cluster *client.VanClient, annotations map[string]string) *v1.StatefulSet {
	name := "nginx-ss"
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-internal",
			Labels: map[string]string{
				"app": name,
			},
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "",
			Ports: []corev1.ServicePort{
				{Name: "web", Port: 8080},
			},
			Selector: map[string]string{
				"app": name,
			},
		},
	}

	// Creating the new service
	svc, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Create(svc)
	assert.Assert(t, err)
	replicas := int32(1)
	ss := &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cluster.Namespace,
			Annotations: annotations,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: v1.StatefulSetSpec{
			ServiceName: name + "-internal",
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
						{Name: "nginx", Image: "nginxinc/nginx-unprivileged:stable-alpine", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 8080}}, ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	ss, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Create(ss)
	assert.Assert(t, err)

	// Wait for statefulSet to be ready
	ss, err = kube.WaitStatefulSetReadyReplicas(ss.Name, cluster.Namespace, 1, cluster.KubeClient, timeout, interval)
	assert.Assert(t, err)

	return ss
}

func createDaemonSet(t *testing.T, cluster *client.VanClient, annotations map[string]string) *v1.DaemonSet {
	name := "nginx-ds"
	ds := &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cluster.Namespace,
			Annotations: annotations,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: v1.DaemonSetSpec{
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
						{Name: "nginx", Image: "nginxinc/nginx-unprivileged:stable-alpine", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 8080}}, ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Create(ds)
	assert.Assert(t, err)

	// Wait for daemonSet to be ready
	ds, err = kube.WaitDaemonSetReady(ds.Name, cluster.Namespace, cluster.KubeClient, timeout, interval)
	assert.Assert(t, err)

	return ds
}

// createService creates a new service at the provided cluster/namespace
// the generated service uses a static selector pointing to the "nginx" pods
func createService(t *testing.T, cluster *client.VanClient, name string, annotations map[string]string) *corev1.Service {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": "nginx",
			},
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "web", Port: 8080},
			},
			Selector: map[string]string{
				"app": "nginx",
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}

	// Creating the new service
	svc, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Create(svc)
	assert.Assert(t, err)

	return svc
}
