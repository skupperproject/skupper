package annotation

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

//
// Modification functions to be used within the test tables
//

// switchProtocols switches the annotated resources protocols
// from http to tcp and vice versa
func switchProtocols(t *testing.T) {

	switchTcpHttp := func(annotations map[string]string) bool {
		if protocol, ok := annotations[types.ProxyQualifier]; ok {
			if protocol == "tcp" {
				annotations[types.ProxyQualifier] = "http"
			} else {
				annotations[types.ProxyQualifier] = "tcp"
			}
			return true
		}
		return false
	}

	for _, cluster := range []*client.VanClient{cluster1, cluster2} {

		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get("nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Retrieving services
		svcList, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(v1.ListOptions{
			LabelSelector: "app=nginx",
		})
		assert.Assert(t, err)

		// Switching protocol
		updateDeployment := switchTcpHttp(dep.Annotations)

		// Iterate through services with the annotation and switch
		svcUpdateList := []v12.Service{}
		for _, svc := range svcList.Items {
			if ok := switchTcpHttp(svc.Annotations); ok {
				svcUpdateList = append(svcUpdateList, svc)
			}
		}

		// Performing updates
		if updateDeployment {
			_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(dep)
			assert.Assert(t, err)
		}

		for _, svc := range svcUpdateList {
			_, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(&svc)
			assert.Assert(t, err)
		}

	}

}

// removeAnnotation will remove annotation from the nginx deployment as
// well as from all services with the label "app=nginx"
func removeAnnotation(t *testing.T) {
	for _, cluster := range []*client.VanClient{cluster1, cluster2} {
		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get("nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Removing annotations and updating
		dep.Annotations = map[string]string{}
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(dep)
		assert.Assert(t, err)

		// Retrieving services
		svcList, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(v1.ListOptions{
			LabelSelector: "app=nginx",
		})
		assert.Assert(t, err)

		// Iterate through services removing annotation and performing the udpate
		for _, svc := range svcList.Items {
			svc.Annotations = map[string]string{}
			_, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(&svc)
			assert.Assert(t, err)
		}
	}
}

// addAnnotation adds the default annotations to the nginx deployment
// as well as for the two services (the one without target and the other
// that uses a target address).
// For more info, see: DeployResources
func addAnnotation(t *testing.T) {
	for i, cluster := range []*client.VanClient{cluster1, cluster2} {
		clusterIdx := i + 1

		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get("nginx", v1.GetOptions{})
		assert.Assert(t, err)
		dep.Annotations = map[string]string{}

		// Retrieving services
		svcNoTarget, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Get(fmt.Sprintf("nginx-%d-svc-exp-notarget", clusterIdx), v1.GetOptions{})
		assert.Assert(t, err)
		svcNoTarget.Annotations = map[string]string{}
		svcTarget, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Get(fmt.Sprintf("nginx-%d-svc-target", clusterIdx), v1.GetOptions{})
		assert.Assert(t, err)
		svcTarget.Annotations = map[string]string{}

		// Populating default annotations
		populateAnnotations(clusterIdx, dep.Annotations, svcNoTarget.Annotations, svcTarget.Annotations)

		// Updating deployment
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(dep)
		assert.Assert(t, err)

		// Updating services
		_, err = cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(svcNoTarget)
		assert.Assert(t, err)
		_, err = cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(svcTarget)
		assert.Assert(t, err)
	}
}
