package annotation

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
	"time"
)

const (
	nsCluster1 = "annotation-1"
	nsCluster2 = "annotation-2"
	timeout    = 120 * time.Second
	interval   = 10 * time.Second
)

var (
	clusters           int = 1
	ctx                    = context.Background()
	kubeConfigCluster1 string
	kubeConfigCluster2 string
	cluster1           *client.VanClient
	cluster2           *client.VanClient
	// teardownClusters prevents deleting an existing namespace
	teardownClusters []*client.VanClient
)

// test allows defining the matrix to run the test table
type test struct {
	name                  string
	doc                   string
	modification          func(*testing.T)
	expectedSites         int
	expectedServicesProto map[string]string
}

// Setup simply creates the namespaces using a single
// cluster by default, or using two clusters if at
// least two kubeconfig files have been provided. It
// also initializes the cluster1 and cluster2 variables.
func Setup(t *testing.T) {
	var err error

	// Need at least one public (or a default) cluster
	pubClusters := k8s.KubeConfigFilesCount(t, false, true)
	if pubClusters == 0 {
		t.Skip("no public clusters available")
	}
	if k8s.KubeConfigFilesCount(t, true, true) > 1 {
		clusters = 2
	}
	t.Logf("Setup - using %d clusters", clusters)

	// Initializing clients
	t.Log("Initializing clients")

	// cluster1 will be used as public (connection token will be generated from it)
	kubeConfigCluster1 = k8s.KubeConfigFiles(t, false, true)[0]
	// cluster2 will connect to cluster1 (can be either an edge, a public or the default)
	kubeConfigCluster2 = k8s.KubeConfigFiles(t, true, true)[0]
	cluster1, err = client.NewClient(nsCluster1, "", kubeConfigCluster1)
	assert.Assert(t, err)
	cluster2, err = client.NewClient(nsCluster2, "", kubeConfigCluster2)
	assert.Assert(t, err)

	// Creating namespaces (or failing if already exists or other error occurs)
	t.Log("Creating namespaces")
	_, err = kube.NewNamespace(nsCluster1, cluster1.KubeClient)
	assert.Assert(t, err)
	teardownClusters = append(teardownClusters, cluster1)
	_, err = kube.NewNamespace(nsCluster2, cluster2.KubeClient)
	assert.Assert(t, err)
	teardownClusters = append(teardownClusters, cluster2)
}

// Teardown ensures all resources are removed
// when test completes
func TearDown(t *testing.T) {
	t.Logf("Tearding down - using %d clusters", clusters)
	t.Log("Deleting namespaces")
	for _, cluster := range teardownClusters {
		err := kube.DeleteNamespace(cluster.Namespace, cluster.KubeClient)
		assert.Assert(t, err)
	}
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
// deployment/nginx  ## to both clusters
//   annotations:
//     skupper.io/proxy: tcp
// service/nginx-1-svc-exp-notarget  ## cluster1
//   annotations:
//     skupper.io/proxy: tcp
// service/nginx-1-svc-target  ## cluster1
//   annotations:
//     skupper.io/proxy: http
//     skupper.io/address: nginx-1-svc-exp-target
// service/nginx-2-svc-exp-notarget  ## cluster2
//   annotations:
//     skupper.io/proxy: tcp
// service/nginx-2-svc-target  ## cluster2
//   annotations:
//     skupper.io/proxy: http
//     skupper.io/address: nginx-1-svc-exp-target
//
func DeployResources(t *testing.T) {
	// Deploys a static set of resources
	t.Logf("Deploying resources - using %d clusters", clusters)

	// Deploys the same set of resources against both clusters
	// resources will have index (1 or 2), depending on the
	// cluster they are being deployed to
	for i, cluster := range []*client.VanClient{cluster1, cluster2} {
		clusterIdx := i + 1

		// Annotations (optional) to deployment and services
		depAnnotations := map[string]string{}
		svcNoTargetAnnotations := map[string]string{}
		svcTargetAnnotations := map[string]string{}
		populateAnnotations(clusterIdx, depAnnotations, svcNoTargetAnnotations, svcTargetAnnotations)

		// One single deployment will be created (for the nginx http server)
		createDeployment(t, cluster, depAnnotations)

		// Now create two services. One that does not have a target address,
		// and another that provides a target address.
		createService(t, cluster, fmt.Sprintf("nginx-%d-svc-exp-notarget", clusterIdx), svcNoTargetAnnotations)
		// This service with the target should not be exposed (only the target service will be)
		createService(t, cluster, fmt.Sprintf("nginx-%d-svc-target", clusterIdx), svcTargetAnnotations)
	}

	// Wait for pods to be running
	for _, cluster := range []*client.VanClient{cluster1, cluster2} {
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
func populateAnnotations(clusterIdx int, depAnnotations map[string]string, svcNoTargetAnnotations map[string]string, svcTargetAnnotations map[string]string) {
	// Define a static set of annotations to the deployment
	depAnnotations[types.ProxyQualifier] = "tcp"
	depAnnotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-dep-web", clusterIdx)

	// Set annotations to the service with no target address
	svcNoTargetAnnotations[types.ProxyQualifier] = "tcp"

	// Set annotations to the service with target address
	svcTargetAnnotations[types.ProxyQualifier] = "http"
	svcTargetAnnotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-svc-exp-target", clusterIdx)
}

// CreateVan deploys Skupper to both clusters then generates a
// connection token on cluster1 and connects cluster2 to cluster1
// waiting for the network to be formed (or failing otherwise)
func CreateVan(t *testing.T) {
	var err error
	t.Logf("Creating VAN - using %d clusters", clusters)

	siteConfig := types.SiteConfig{
		Spec: types.SiteConfigSpec{
			EnableController:  true,
			EnableServiceSync: true,
			User:              "admin",
			Password:          "admin",
			ClusterLocal:      false,
		},
	}

	// If using only 1 cluster, set ClusterLocal to True
	if clusters == 1 {
		siteConfig.Spec.ClusterLocal = true
	}
	t.Logf("setting siteConfig to run with ClusterLocal=%v", siteConfig.Spec.ClusterLocal)

	// Creating the router on cluster1
	err = cluster1.RouterCreate(ctx, siteConfig)
	assert.Assert(t, err, "error creating router on cluster1")

	// Creating the router on cluster2
	err = cluster2.RouterCreate(ctx, siteConfig)
	assert.Assert(t, err, "error creating router on cluster2")

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)

	// Creating token and connecting sites
	tokenFile := testPath + "cluster1.yaml"
	err = cluster1.ConnectorTokenCreateFile(ctx, "cluster1", tokenFile)
	assert.Assert(t, err, "unable to create token to cluster1")

	// Connecting cluster2 to cluster1
	secret, err := cluster2.ConnectorCreateFromFile(ctx, tokenFile, types.ConnectorCreateOptions{SkupperNamespace: cluster2.Namespace})
	assert.Assert(t, err, "unable to create connection to cluster1")
	assert.Assert(t, secret != nil)

	// Waiting till skupper is running on both clusters
	for _, c := range []*client.VanClient{cluster1, cluster2} {
		t.Logf("waiting for Skupper sites to be connected at: %s", c.Namespace)
		err = utils.Retry(interval, 20, func() (bool, error) {
			rir, _ := c.RouterInspect(ctx)
			connected := 0
			if rir != nil {
				connected = rir.Status.ConnectedSites.Total
			}
			return connected == 1, nil
		})
		assert.Assert(t, err, "timed out waiting on Skupper sites to connect at: %s", c.Namespace)
	}
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
