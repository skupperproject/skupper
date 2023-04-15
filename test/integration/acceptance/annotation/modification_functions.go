package annotation

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"
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
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Retrieving the statefulset
		ss, err := cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Get(context.TODO(), "nginx-ss", v1.GetOptions{})
		assert.Assert(t, err)

		// Retrieving the statefulset
		ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Get(context.TODO(), "nginx-ds", v1.GetOptions{})
		assert.Assert(t, err)

		// Retrieving services
		svcList, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(context.TODO(), v1.ListOptions{
			LabelSelector: "app=nginx",
		})
		assert.Assert(t, err)

		// Switching protocol
		updateDeployment := switchTcpHttp(dep.Annotations)
		updateStatefulSet := switchTcpHttp(ss.Annotations)
		updateDaemonSet := switchTcpHttp(ds.Annotations)

		// Iterate through services with the annotation and switch
		var svcUpdateList []corev1.Service
		for _, svc := range svcList.Items {
			if ok := switchTcpHttp(svc.Annotations); ok {
				svcUpdateList = append(svcUpdateList, svc)
			}
		}

		// Performing updates
		if updateDeployment {
			_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(context.TODO(), dep, v1.UpdateOptions{})
			assert.Assert(t, err)
		}
		if updateStatefulSet {
			_, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Update(context.TODO(), ss, v1.UpdateOptions{})
			assert.Assert(t, err)
		}
		if updateDaemonSet {
			_, err = cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Update(context.TODO(), ds, v1.UpdateOptions{})
			assert.Assert(t, err)
		}

		for _, svc := range svcUpdateList {
			_, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(context.TODO(), &svc, v1.UpdateOptions{})
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
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Removing annotations and updating
		delete(dep.Annotations, types.ProxyQualifier)
		delete(dep.Annotations, types.AddressQualifier)
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(context.TODO(), dep, v1.UpdateOptions{})
		assert.Assert(t, err)

		// Retrieving the statefulSet
		ss, err := cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Get(context.TODO(), "nginx-ss", v1.GetOptions{})
		assert.Assert(t, err)

		// Removing annotations and updating
		delete(ss.Annotations, types.ProxyQualifier)
		delete(ss.Annotations, types.AddressQualifier)
		_, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Update(context.TODO(), ss, v1.UpdateOptions{})
		assert.Assert(t, err)

		// Retrieving the daemonSet
		ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Get(context.TODO(), "nginx-ds", v1.GetOptions{})
		assert.Assert(t, err)

		// Removing annotations and updating
		delete(ds.Annotations, types.ProxyQualifier)
		delete(ds.Annotations, types.AddressQualifier)
		_, err = cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Update(context.TODO(), ds, v1.UpdateOptions{})
		assert.Assert(t, err)

		// Retrieving services
		svcList, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(context.TODO(), v1.ListOptions{
			LabelSelector: "app=nginx",
		})
		assert.Assert(t, err)

		// Iterate through services removing annotation and performing the update
		for _, svc := range svcList.Items {
			delete(svc.Annotations, types.ProxyQualifier)
			delete(svc.Annotations, types.AddressQualifier)
			_, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(context.TODO(), &svc, v1.UpdateOptions{})
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
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)
		dep.Annotations = map[string]string{}

		// Retrieving the statefulset
		ss, err := cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Get(context.TODO(), "nginx-ss", v1.GetOptions{})
		assert.Assert(t, err)
		ss.Annotations = map[string]string{}

		// Retrieving the daemonset
		ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Get(context.TODO(), "nginx-ds", v1.GetOptions{})
		assert.Assert(t, err)
		ds.Annotations = map[string]string{}

		// Retrieving services
		svcNoTarget, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Get(context.TODO(), fmt.Sprintf("nginx-%d-svc-exp-notarget", clusterIdx), v1.GetOptions{})
		assert.Assert(t, err)
		svcNoTarget.Annotations = map[string]string{}
		svcTarget, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Get(context.TODO(), fmt.Sprintf("nginx-%d-svc-target", clusterIdx), v1.GetOptions{})
		assert.Assert(t, err)
		svcTarget.Annotations = map[string]string{}

		// Populating default annotations
		populateAnnotations(clusterIdx, dep.Annotations, svcNoTarget.Annotations, svcTarget.Annotations, ss.Annotations, ds.Annotations)

		// Updating deployment
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(context.TODO(), dep, v1.UpdateOptions{})
		assert.Assert(t, err)

		// Updating statefulSet
		_, err = cluster.KubeClient.AppsV1().StatefulSets(cluster.Namespace).Update(context.TODO(), ss, v1.UpdateOptions{})
		assert.Assert(t, err)

		// Updating daemonSet
		_, err = cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Update(context.TODO(), ds, v1.UpdateOptions{})
		assert.Assert(t, err)

		// Updating services
		_, err = cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(context.TODO(), svcNoTarget, v1.UpdateOptions{})
		assert.Assert(t, err)
		_, err = cluster.KubeClient.CoreV1().Services(cluster.Namespace).Update(context.TODO(), svcTarget, v1.UpdateOptions{})
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
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)
		log.Printf("Deployment: %s - Annotations: %s", dep.Name, dep.Annotations)
		if len(dep.Annotations) > 0 {
			i++
		}

		// Retrieving services
		svcList, _ := cluster.KubeClient.CoreV1().Services(cluster.Namespace).List(context.TODO(), v1.ListOptions{
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

// AddPorts adds extra ports to already annotated resources
func AddPorts(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	for _, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Updating ports (adding 8888:8080 and 9999:8080)
		dep.Annotations[types.PortQualifier] = fmt.Sprintf("8080:8080,9090:8080,8888:8080,9999:8080")

		// Updating deployment
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(context.TODO(), dep, v1.UpdateOptions{})
		assert.Assert(t, err)
	}
}

// RemovePorts remove the extra ports from already annotated resources
func RemovePorts(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	for _, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Updating ports (adding 8888:8080 and 9999:8080)
		dep.Annotations[types.PortQualifier] = fmt.Sprintf("8080:8080,9090:8080,8888:8080")

		// Updating deployment
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(context.TODO(), dep, v1.UpdateOptions{})
		assert.Assert(t, err)
	}
}

// ChangeAddress target address to use not owned service
func ChangeAddress(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	for i, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		clusterIdx := i + 1

		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Updating address to clash with not owned service
		dep.Annotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-dep-not-owned", clusterIdx)

		// Updating deployment
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(context.TODO(), dep, v1.UpdateOptions{})
		assert.Assert(t, err)
	}

	// now validates that the original selector has been restored
	origSelector := map[string]string{
		"application":          types.TransportDeploymentName,
		"skupper.io/component": "router",
	}

	t.Logf("Waiting till router selectors in the not owned service becomes stable")
	waitSelectorsStable(t, testRunner, origSelector)

}

// RevertAddress target address to use owned service
func RevertAddress(t *testing.T, testRunner base.ClusterTestRunner) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	for i, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		clusterIdx := i + 1

		// Retrieving the deployment
		dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), "nginx", v1.GetOptions{})
		assert.Assert(t, err)

		// Updating address to clash with not owned service
		dep.Annotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-dep-web", clusterIdx)

		// Updating deployment
		_, err = cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Update(context.TODO(), dep, v1.UpdateOptions{})
		assert.Assert(t, err)
	}

	// now validates that the original selector has been restored
	origSelector := map[string]string{
		"app": "nginx",
	}

	t.Logf("Waiting till restored selectors in the not owned service becomes stable")
	waitSelectorsStable(t, testRunner, origSelector)
}

func waitSelectorsStable(t *testing.T, testRunner base.ClusterTestRunner, origSelector map[string]string) {
	const maxChanges = 5
	const stableTime = time.Second * 30

	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	for i, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		clusterIdx := i + 1

		serviceName := fmt.Sprintf("nginx-%d-dep-not-owned", clusterIdx)

		changes := 0
		doneCh := make(chan struct{})
		svcInformer := corev1informer.NewFilteredServiceInformer(
			cluster.KubeClient,
			cluster.Namespace,
			time.Second*30,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			internalinterfaces.TweakListOptionsFunc(func(options *v1.ListOptions) {
				options.FieldSelector = "metadata.name=" + serviceName
			}))
		svcInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldSvc := oldObj.(*corev1.Service)
				newSvc := newObj.(*corev1.Service)
				if !reflect.DeepEqual(newSvc.Spec.Selector, origSelector) {
					log.Printf("selectors changed - old: %v - new: %v", oldSvc.Spec.Selector, newSvc.Spec.Selector)
					changes += 1
				}
				if changes == maxChanges {
					close(doneCh)
				}
			},
		})
		go svcInformer.Run(doneCh)

		ctx, cn := context.WithTimeout(context.Background(), stableTime)
		defer cn()

		select {
		case <-doneCh:
			assert.Assert(t, fmt.Errorf("selectors are unstable - changed %d times", changes))
		case <-ctx.Done():
			close(doneCh)
			svc, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Get(context.Background(), serviceName, v1.GetOptions{})
			assert.Assert(t, err)
			assert.DeepEqual(t, origSelector, svc.Spec.Selector)
			log.Printf("selectors are stable!")
		}
	}
}
