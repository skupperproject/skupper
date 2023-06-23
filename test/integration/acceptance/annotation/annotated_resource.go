package annotation

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	timeout  = 120 * time.Second
	interval = 5 * time.Second
)

var (
	servicesMap = map[string][]string{}
)

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
//
//	annotations:
//	  skupper.io/proxy: tcp
//	  skupper.io/address: nginx-1-dep-web
//	  skupper.io/port: 8080:8080,9090:8080
//
// statefulset/nginx  ## cluster1
//
//	annotations:
//	  skupper.io/proxy: tcp
//	  skupper.io/address: nginx-1-ss-web
//
// daemonset/nginx  ## cluster1
//
//	annotations:
//	  skupper.io/proxy: tcp
//	  skupper.io/address: nginx-1-ds-web
//
// service/nginx-1-svc-exp-notarget  ## cluster1
//
//	annotations:
//	  skupper.io/proxy: tcp
//
// service/nginx-1-svc-target  ## cluster1
//
//	annotations:
//	  skupper.io/proxy: http
//	  skupper.io/address: nginx-1-svc-exp-target
//
// deployment/nginx  ## cluster2
//
//	annotations:
//	  skupper.io/proxy: tcp
//	  skupper.io/address: nginx-2-dep-web
//	  skupper.io/port: 8080:8080,9090:8080
//
// statefulset/nginx  ## cluster2
//
//	annotations:
//	  skupper.io/proxy: tcp
//	  skupper.io/address: nginx-2-ss-web
//
// daemonset/nginx  ## cluster2
//
//	annotations:
//	  skupper.io/proxy: tcp
//	  skupper.io/address: nginx-2-ds-web
//
// service/nginx-2-svc-exp-notarget  ## cluster2
//
//	annotations:
//	  skupper.io/proxy: tcp
//
// service/nginx-2-svc-target  ## cluster2
//
//	annotations:
//	  skupper.io/proxy: http
//	  skupper.io/address: nginx-1-svc-exp-target
func DeployResources(testRunner base.ClusterTestRunner) error {
	// Deploys a static set of resources
	log.Printf("Deploying resources")

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

		// Create a service without annotations to be taken by Skupper as a deployment will be annotated with this service address
		if _, err := createService(cluster, fmt.Sprintf("nginx-%d-dep-not-owned", clusterIdx), map[string]string{}); err != nil {
			return err
		}
		// One single deployment will be created (for the nginx http server)
		if _, err := createDeployment(cluster, depAnnotations); err != nil {
			return err
		}
		if _, err := createStatefulSet(cluster, statefulSetAnnotations); err != nil {
			return err
		}
		if _, err := createDaemonSet(cluster, daemonSetAnnotations); err != nil {
			return err
		}

		// Now create two services. One that does not have a target address,
		// and another that provides a target address.
		if _, err := createService(cluster, fmt.Sprintf("nginx-%d-svc-exp-notarget", clusterIdx), svcNoTargetAnnotations); err != nil {
			return err
		}
		// This service with the target should not be exposed (only the target service will be)
		if _, err := createService(cluster, fmt.Sprintf("nginx-%d-svc-target", clusterIdx), svcTargetAnnotations); err != nil {
			return err
		}
	}

	// Wait for pods to be running
	for _, cluster := range []*client.VanClient{pub.VanClient, prv.VanClient} {
		log.Printf("waiting on pods to be running on %s", cluster.Namespace)
		// Get all pod names
		podList, err := cluster.KubeClient.CoreV1().Pods(cluster.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(podList.Items) == 0 {
			return fmt.Errorf("no pods running")
		}

		for _, pod := range podList.Items {
			_, err := kube.WaitForPodStatus(cluster.Namespace, cluster.KubeClient, pod.Name, corev1.PodRunning, timeout, interval)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func UndeployResources(testRunner base.ClusterTestRunner) error {
	// Undeploy resources
	log.Printf("Undeploying resources")

	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	// Removing deployed resources
	for _, cluster := range []*base.ClusterContext{pub, prv} {
		cli := cluster.VanClient.KubeClient
		log.Printf("removing deployment 'nginx' from: %s", cluster.Namespace)
		if err := cli.AppsV1().Deployments(cluster.Namespace).Delete(context.TODO(), "nginx", metav1.DeleteOptions{}); err != nil {
			return err
		}
		log.Printf("removing daemonset 'nginx-ds' from: %s", cluster.Namespace)
		if err := cli.AppsV1().DaemonSets(cluster.Namespace).Delete(context.TODO(), "nginx-ds", metav1.DeleteOptions{}); err != nil {
			return err
		}
		log.Printf("removing statefulset 'nginx-ss' from: %s", cluster.Namespace)
		if err := cli.AppsV1().StatefulSets(cluster.Namespace).Delete(context.TODO(), "nginx-ss", metav1.DeleteOptions{}); err != nil {
			return err
		}
		for _, svc := range servicesMap[cluster.Namespace] {
			log.Printf("removing service '%s' from: %s", svc, cluster.Namespace)
			if err := cli.CoreV1().Services(cluster.Namespace).Delete(context.TODO(), svc, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}

// populateAnnotations annotates the provide maps with static
// annotations for the deployment, and for each of the services
func populateAnnotations(clusterIdx int, depAnnotations map[string]string, svcNoTargetAnnotations map[string]string, svcTargetAnnotations map[string]string,
	statefulSetAnnotations map[string]string, daemonSetAnnotations map[string]string) {
	// Define a static set of annotations to the deployment
	depAnnotations[types.ProxyQualifier] = "tcp"
	depAnnotations[types.AddressQualifier] = fmt.Sprintf("nginx-%d-dep-web", clusterIdx)
	depAnnotations[types.PortQualifier] = fmt.Sprintf("8080:8080,9090:8080")

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

// createDeployment creates a pre-defined nginx Deployment at the given
// cluster and namespace. Reason for using it is that it is a tiny image
// and allows tests to validate traffic flowing using both http and tcp bridges.
func createDeployment(cluster *client.VanClient, annotations map[string]string) (*v1.Deployment, error) {
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
						{Name: "nginx", Image: "quay.io/dhashimo/nginx-unprivileged:stable-alpine", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 8080}}, ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Create(context.TODO(), dep, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Wait for deployment to be ready
	dep, err = kube.WaitDeploymentReadyReplicas(dep.Name, cluster.Namespace, 1, cluster.KubeClient, timeout, interval)
	if err != nil {
		return nil, err
	}

	return dep, nil
}

func createStatefulSet(cluster *client.VanClient, annotations map[string]string) (*v1.StatefulSet, error) {
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
	svc, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
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
						{Name: "nginx", Image: "quay.io/dhashimo/nginx-unprivileged:stable-alpine", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 8080}}, ImagePullPolicy: corev1.PullIfNotPresent},
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
	ss, err = kube.WaitStatefulSetReadyReplicas(ss.Name, cluster.Namespace, 1, cluster.KubeClient, timeout, interval)
	if err != nil {
		return nil, err
	}

	return ss, nil
}

func createDaemonSet(cluster *client.VanClient, annotations map[string]string) (*v1.DaemonSet, error) {
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
						{Name: "nginx", Image: "quay.io/dhashimo/nginx-unprivileged:stable-alpine", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 8080}}, ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	ds, err := cluster.KubeClient.AppsV1().DaemonSets(cluster.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Wait for daemonSet to be ready
	ds, err = kube.WaitDaemonSetReady(ds.Name, cluster.Namespace, cluster.KubeClient, timeout, interval)
	if err != nil {
		return nil, err
	}

	return ds, nil
}

// createService creates a new service at the provided cluster/namespace
// the generated service uses a static selector pointing to the "nginx" pods
func createService(cluster *client.VanClient, name string, annotations map[string]string) (*corev1.Service, error) {

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
	svc, err := cluster.KubeClient.CoreV1().Services(cluster.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Populate services map by namespace
	services := []string{svc.Name}
	var ok bool
	if services, ok = servicesMap[cluster.Namespace]; ok {
		services = append(services, svc.Name)
	}
	servicesMap[cluster.Namespace] = services

	return svc, nil
}
