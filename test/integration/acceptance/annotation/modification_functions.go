package annotation

import (
	"fmt"
	"log"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// Modification functions to be used within the test tables
//

// SwitchProtocols switches the annotated resources protocols
// from http to tcp and vice versa
func SwitchProtocols(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

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

	for _, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {

		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get("nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Retrieving the statefulset
		ss, err := cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Get("nginx-ss", v1.GetOptions{})
		assert.Assert(t, err)

		// Retrieving the statefulset
		ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Get("nginx-ds", v1.GetOptions{})
		assert.Assert(t, err)

		// Retrieving services
		svcList, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(v1.ListOptions{
			LabelSelector: "app=nginx",
		})
		assert.Assert(t, err)

		// Switching protocol
		updateDeployment := switchTcpHttp(dep.Annotations)
		updateStatefulSet := switchTcpHttp(ss.Annotations)
		updateDaemonSet := switchTcpHttp(ds.Annotations)

		// Iterate through services with the annotation and switch
		var svcUpdateList []v12.Service
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
		if updateStatefulSet {
			_, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Update(ss)
			assert.Assert(t, err)
		}
		if updateDaemonSet {
			_, err = cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Update(ds)
			assert.Assert(t, err)
		}

		for _, svc := range svcUpdateList {
			_, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(&svc)
			assert.Assert(t, err)
		}

	}

}

// RemoveAnnotation will remove annotation from the nginx deployment as
// well as from all services with the label "app=nginx"
func RemoveAnnotation(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	for _, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get("nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Removing annotations and updating
		delete(dep.Annotations, types.ProxyQualifier)
		delete(dep.Annotations, types.AddressQualifier)
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(dep)
		assert.Assert(t, err)

		// Retrieving the statefulSet
		ss, err := cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Get("nginx-ss", v1.GetOptions{})
		assert.Assert(t, err)

		// Removing annotations and updating
		delete(ss.Annotations, types.ProxyQualifier)
		delete(ss.Annotations, types.AddressQualifier)
		_, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Update(ss)
		assert.Assert(t, err)

		// Retrieving the daemonSet
		ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Get("nginx-ds", v1.GetOptions{})
		assert.Assert(t, err)

		// Removing annotations and updating
		delete(ds.Annotations, types.ProxyQualifier)
		delete(ds.Annotations, types.AddressQualifier)
		_, err = cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Update(ds)
		assert.Assert(t, err)

		// Retrieving services
		svcList, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(v1.ListOptions{
			LabelSelector: "app=nginx",
		})
		assert.Assert(t, err)

		// Iterate through services removing annotation and performing the update
		for _, svc := range svcList.Items {
			delete(svc.Annotations, types.ProxyQualifier)
			delete(svc.Annotations, types.AddressQualifier)
			_, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(&svc)
			assert.Assert(t, err)
		}
	}
}

// AddAnnotation adds the default annotations to the nginx deployment
// as well as for the two services (the one without target and the other
// that uses a target address).
// For more info, see: DeployResources
func AddAnnotation(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	for i, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		clusterIdx := i + 1

		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get("nginx", v1.GetOptions{})
		assert.Assert(t, err)
		dep.Annotations = map[string]string{}

		// Retrieving the statefulset
		ss, err := cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Get("nginx-ss", v1.GetOptions{})
		assert.Assert(t, err)
		ss.Annotations = map[string]string{}

		// Retrieving the daemonset
		ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Get("nginx-ds", v1.GetOptions{})
		assert.Assert(t, err)
		ds.Annotations = map[string]string{}

		// Retrieving services
		svcNoTarget, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Get(fmt.Sprintf("nginx-%d-svc-exp-notarget", clusterIdx), v1.GetOptions{})
		assert.Assert(t, err)
		svcNoTarget.Annotations = map[string]string{}
		svcTarget, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Get(fmt.Sprintf("nginx-%d-svc-target", clusterIdx), v1.GetOptions{})
		assert.Assert(t, err)
		svcTarget.Annotations = map[string]string{}

		// Populating default annotations
		populateAnnotations(clusterIdx, dep.Annotations, svcNoTarget.Annotations, svcTarget.Annotations, ss.Annotations, ds.Annotations)

		// Updating deployment
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(dep)
		assert.Assert(t, err)

		// Updating statefulSet
		_, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Update(ss)
		assert.Assert(t, err)

		// Updating daemonSet
		_, err = cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Update(ds)
		assert.Assert(t, err)

		// Updating services
		_, err = cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(svcNoTarget)
		assert.Assert(t, err)
		_, err = cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(svcTarget)
		assert.Assert(t, err)
	}
}

// DebugAnnotatedResources prints current status for the exposed resources
func DebugAnnotatedResources(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	log.Printf("Debugging exposed resources (by annotation):")
	i := 0
	for _, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get("nginx", v1.GetOptions{})
		assert.Assert(t, err)
		log.Printf("Deployment: %s - Annotations: %s", dep.Name, dep.Annotations)
		if len(dep.Annotations) > 0 {
			i++
		}

		// Retrieving services
		svcList, _ := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(v1.ListOptions{
			LabelSelector: "app=nginx",
		})

		for _, svc := range svcList.Items {
			log.Printf("Service   : %s - Annotations: %s", svc.Name, svc.Annotations)
			if _, ok := svc.Annotations[types.ProxyQualifier]; ok {
				i++
			}
		}

	}
	log.Printf("Number of exposed resources found (with %s annotation): %d", types.ProxyQualifier, i)
}
