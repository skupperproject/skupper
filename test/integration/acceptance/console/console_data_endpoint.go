package console

import (
	"context"

	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "os"
	"testing"
)

var (
	backsvc = types.ServiceInterface{
		Address:  "hello-world-backend",
		Protocol: "http",
		Ports:    []int{8080},
	}

	frontsvc = types.ServiceInterface{
		Address:  "hello-world-frontend",
		Protocol: "http",
		Ports:    []int{8080},
	}
)

type HttpRequestFromConsole struct {
	Requests int
	BytesIn  int
	BytesOut int
}

// Create the deployment for the Frontend in public namespace
func CreateFrontendDeployment(t *testing.T, cluster *client.VanClient) {
	name := "hello-world-frontend"
	replicas := int32(1)
	dep := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    map[string]string{"app": name},
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
						{
							Name:            name,
							Image:           "quay.io/skupper/" + name,
							ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Create(dep)
	assert.Assert(t, err)
}

// Create the deployment for the Backtend in public namespace
func CreateBackendDeployment(t *testing.T, cluster *client.VanClient) {
	name := "hello-world-backend"
	replicas := int32(1)
	dep := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    map[string]string{"app": name},
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
						{
							Name:            name,
							Image:           "quay.io/skupper/" + name,
							ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Create(dep)
	assert.Assert(t, err)
}

func Setup(ctx context.Context, t *testing.T, r base.ClusterTestRunner) {
	log.Warn("Setting up deployments for console_data_endpoint test")
	publicCluster, _ := r.GetPublicContext(1)
	privateCluster, _ := r.GetPrivateContext(1)

	// Create the frontend Deployment
	CreateFrontendDeployment(t, publicCluster.VanClient)

	// Create the backend Deployment
	CreateBackendDeployment(t, privateCluster.VanClient)

	err := privateCluster.VanClient.ServiceInterfaceCreate(ctx, &backsvc)
	assert.Assert(t, err)

	err = privateCluster.VanClient.ServiceInterfaceBind(ctx, &backsvc, "deployment", "hello-world-backend", "http", map[int]int{8080: 8080})
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(publicCluster.Namespace, publicCluster.VanClient.KubeClient, "hello-world-backend")
	assert.Assert(t, err)

	err = publicCluster.VanClient.ServiceInterfaceCreate(ctx, &frontsvc)
	assert.Assert(t, err)

	err = publicCluster.VanClient.ServiceInterfaceBind(ctx, &frontsvc, "deployment", "hello-world-frontend", "http", map[int]int{8080: 8080})
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(publicCluster.Namespace, publicCluster.VanClient.KubeClient, "hello-world-frontend")
	assert.Assert(t, err)
}

// Remove the namespaces
func TearDown(t *testing.T, r base.ClusterTestRunner) error {

	publicCluster, _ := r.GetPublicContext(1)
	privateCluster, _ := r.GetPrivateContext(1)

	// Delete the frontend deployment
	assert.Assert(t, publicCluster.VanClient.KubeClient.AppsV1().Deployments(publicCluster.Namespace).Delete(frontsvc.Address, &metav1.DeleteOptions{}))
	// Delete the backend deployment
	assert.Assert(t, privateCluster.VanClient.KubeClient.AppsV1().Deployments(privateCluster.Namespace).Delete(backsvc.Address, &metav1.DeleteOptions{}))

	return nil
}
