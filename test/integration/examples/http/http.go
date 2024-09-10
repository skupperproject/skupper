package http

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// test table
type test struct {
	name            string
	doc             string
	cluster         *base.ClusterContext
	numOfWorkers    string
	durationOfTests string
	jobName         string
	targetURL       string
}

func int32Ptr(i int32) *int32 { return &i }

var service = types.ServiceInterface{
	Address:  "nginx1",
	Protocol: "http",
	Ports:    []int{8080},
}

var http2service = types.ServiceInterface{
	Address:  "nghttp2",
	Protocol: "http2",
	Ports:    []int{8443},
}

var http2TlsService = types.ServiceInterface{
	Address:          "nghttp2tls",
	Protocol:         "http2",
	Ports:            []int{8443},
	TlsCredentials:   "skupper-tls-nghttp2tls",
	TlsCertAuthority: types.ServiceClientSecret,
}

var http2TcpTlsService = types.ServiceInterface{
	Address:          "nghttp2tcptls",
	Protocol:         "tcp",
	Ports:            []int{8443},
	TlsCredentials:   "skupper-tls-nghttp2tcptls",
	TlsCertAuthority: "skupper-service-client",
}

var http1TlsService = types.ServiceInterface{
	Address:          "nghttp1tls",
	Protocol:         "http",
	Ports:            []int{8443},
	TlsCredentials:   "skupper-tls-nghttp1tls",
	TlsCertAuthority: types.ServiceClientSecret,
}

var nginxDep = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nginx1",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nginx1"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nginx1",
				},
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "nginx1",
						Image:           "quay.io/nginx/nginx-unprivileged:stable-alpine",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "http",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8080,
							},
						},
					},
				},
			},
		},
	},
}

var nghttp2Dep = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp2",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp2"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp2",
				},
			},
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "index-html",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "index-html",
								},
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nghttp2",
						Image:           "quay.io/skupper/nghttp2",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp2",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8443,
							},
						},
						Command: []string{
							"nghttpd",
							"--no-tls",
							"-v",
							"8443",
							"-d",
							"/webroot/",
						},
						VolumeMounts: []apiv1.VolumeMount{
							{Name: "index-html", MountPath: "/webroot", ReadOnly: true},
						},
					},
				},
			},
		},
	},
}

var nghttp2TlsDep = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp2tls",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp2tls"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp2tls",
				},
			},
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "index-html",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "index-html",
								},
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nghttp2tls",
						Image:           "quay.io/skupper/nghttp2",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp2tls",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8443,
							},
						},
						Command: []string{
							"nghttpd",
							"--no-tls",
							"-v",
							"8443",
							"-d",
							"/webroot/",
						},
						VolumeMounts: []apiv1.VolumeMount{
							{Name: "index-html", MountPath: "/webroot", ReadOnly: true},
						},
					},
				},
			},
		},
	},
}

var nghttp2TcpTlsDep = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp2tcptls",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp2tcptls"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp2tcptls",
				},
			},
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "index-html",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "index-html",
								},
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nghttp2tcptls",
						Image:           "quay.io/skupper/nghttp2",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp2tcptls",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8443,
							},
						},
						Command: []string{
							"nghttpd",
							"--no-tls",
							"-v",
							"8443",
							"-d",
							"/webroot/",
						},
						VolumeMounts: []apiv1.VolumeMount{
							{Name: "index-html", MountPath: "/webroot", ReadOnly: true},
						},
					},
				},
			},
		},
	},
}

var nghttp1TlsDep = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp1tls",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp1tls"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp1tls",
				},
			},
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "index-html",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "index-html",
								},
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nghttp1tls",
						Image:           "quay.io/nginx/nginx-unprivileged:stable-alpine",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp1tls",
								ContainerPort: 8443,
							},
						},
						VolumeMounts: []apiv1.VolumeMount{
							{Name: "index-html", MountPath: "/etc/nginx/html", ReadOnly: true},
						},
					},
				},
			},
		},
	},
}

var nghttp2TlsDepWithCertFiles = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp2tls",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp2tls"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp2tls",
				},
			},
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "index-html",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "index-html",
								},
							},
						},
					},
					{
						Name: "certs",
						VolumeSource: apiv1.VolumeSource{
							Secret: &apiv1.SecretVolumeSource{
								SecretName: "skupper-tls-nghttp2tls",
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nghttp2tls",
						Image:           "quay.io/skupper/nghttp2",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp2tls",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8443,
							},
						},
						Command: []string{
							"nghttpd",
							"-v",
							"-d",
							"/webroot/",
							"8443",
							"/certs/tls.key",
							"/certs/tls.crt",
						},
						VolumeMounts: []apiv1.VolumeMount{
							{Name: "index-html", MountPath: "/webroot", ReadOnly: true},
							{Name: "certs", MountPath: "/certs", ReadOnly: true},
						},
					},
				},
			},
		},
	},
}

var nghttp2tcpTlsDepWithCertFiles = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp2tcptls",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp2tcptls"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp2tcptls",
				},
			},
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "index-html",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "index-html",
								},
							},
						},
					},
					{
						Name: "certs",
						VolumeSource: apiv1.VolumeSource{
							Secret: &apiv1.SecretVolumeSource{
								SecretName: "skupper-tls-nghttp2tcptls",
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nghttp2tcptls",
						Image:           "quay.io/skupper/nghttp2",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp2tcptls",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8443,
							},
						},
						Command: []string{
							"nghttpd",
							"-v",
							"-d",
							"/webroot/",
							"8443",
							"/certs/tls.key",
							"/certs/tls.crt",
						},
						VolumeMounts: []apiv1.VolumeMount{
							{Name: "index-html", MountPath: "/webroot", ReadOnly: true},
							{Name: "certs", MountPath: "/certs", ReadOnly: true},
						},
					},
				},
			},
		},
	},
}

var nghttp1TlsConfigMap = &apiv1.ConfigMap{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp1configmap",
	},
	Data: map[string]string{
		"nginx.conf": `worker_processes 1;

pid /tmp/nginx.pid;

events {
  worker_connections 1024;
}

http {
  server {
    listen 8443 ssl;
    server_name example.com;

    ssl_certificate /certs/tls.crt;
    ssl_certificate_key /certs/tls.key;
  }
}`,
	},
}

var nghttp1TlsDepWithCertFiles = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "nghttp1tls",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "nghttp1tls"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "nghttp1tls",
				},
			},
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "index-html",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "index-html",
								},
							},
						},
					},
					{
						Name: "nghttp1configmap",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "nghttp1configmap",
								},
							},
						},
					},
					{
						Name: "certs",
						VolumeSource: apiv1.VolumeSource{
							Secret: &apiv1.SecretVolumeSource{
								SecretName: "skupper-tls-nghttp1tls",
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nghttp1tls",
						Image:           "quay.io/nginx/nginx-unprivileged:stable-alpine",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "nghttp1tls",
								ContainerPort: 8443,
							},
						},
						VolumeMounts: []apiv1.VolumeMount{
							{Name: "index-html", MountPath: "/etc/nginx/html", ReadOnly: true},
							{Name: "certs", MountPath: "/certs", ReadOnly: true},
							{Name: "nghttp1configmap", MountPath: "/etc/nginx/nginx.conf", SubPath: "nginx.conf"},
						},
					},
				},
			},
		},
	},
}

// HTTP2 Load Job
var h2loadJob = &batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Name: "h2load",
		// Namespace: namespace,
	},
	Spec: batchv1.JobSpec{
		BackoffLimit: int32Ptr(3),
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "h2load",
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:  "h2load",
						Image: "quay.io/skupper/nghttp2",
						// Command: []string{"h2load", "-n10", "-c10", "-m10", "http://nghttp2:8443"},
						Command: []string{"h2load", "-n1000", "-c1", "-m1", "http://nghttp2:8443"},
						Env: []apiv1.EnvVar{
							{Name: "JOB", Value: "h2load"},
						},
						ImagePullPolicy: apiv1.PullAlways,
					},
				},
				RestartPolicy: apiv1.RestartPolicyNever,
			},
		},
	},
}

// Base HTTP1 concurrent requests with Hey
var h1HeyBaseJob = &batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Name: "h1heybasejob",
		// Namespace: namespace,
	},
	Spec: batchv1.JobSpec{
		BackoffLimit: int32Ptr(3),
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "h1heybasejob",
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "h1heybase",
						Image:           "quay.io/skupper/hey",
						Command:         []string{"hey_linux_amd64"},
						ImagePullPolicy: apiv1.PullAlways,
					},
				},
				RestartPolicy: apiv1.RestartPolicyNever,
			},
		},
	},
}

// This is required to avoid test flakiness while #1208 is not fixed.  It will wait for the
// secret to be created up to a maximum time.
//
// It returns nothing and ignores any errors that occur, leaving any inconsistencies to be
// detected later on by other code
func waitSecret(cctx *base.ClusterContext, secretName string) {
	// The check below is required while https://github.com/skupperproject/skupper/issues/1208
	// is not fixed; both the ServiceInterfaceCreate above and ServiceInterfaceBind below may
	// create the secret, and sometimes they race on it, making the test flaky.
	err := utils.Retry(
		time.Second*3,
		20,
		func() (bool, error) {
			_, err := cctx.VanClient.KubeClient.CoreV1().Secrets(cctx.Namespace).Get(
				context.Background(),
				secretName,
				metav1.GetOptions{},
			)
			if err == nil {
				return true, nil
			}
			if errors.IsNotFound(err) {
				fmt.Printf("TLS Secret %q not yet created.  See #1208\n", secretName)
				return false, nil
			}
			return false, err
		},
	)
	if err != nil {
		fmt.Printf("Secret retrieval failed: %v\n", err)
		// We do not stop for this, as this test is just to avoid flakiness on the
		// test; if whatever failed above is reason for an actual test failure,
		// it will fail on the continuation of the actual test, below
	}
}

// Updating a deployment may cause Skupper to trigger an asynchronous operation that updates
// the configmap skupper-internal.  That may happen at the same time as another synchronous
// operation down the road, causing it to fail (see #1153).
//
// This function executes the requested operation, but it first gets the ResourceVersion of
// the configmap, and then waits until it changes, or a maximum wait time is reached.
func updateDeployment(cctx *base.ClusterContext, ctx context.Context, deploy *appsv1.Deployment) error {
	orig, err := cctx.VanClient.KubeClient.CoreV1().ConfigMaps(cctx.Namespace).Get(ctx, "skupper-internal", metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, err = cctx.VanClient.KubeClient.AppsV1().Deployments(cctx.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	err = utils.Retry(
		time.Millisecond*600,
		100,
		func() (bool, error) {
			cm, err := cctx.VanClient.KubeClient.CoreV1().ConfigMaps(cctx.Namespace).Get(ctx, "skupper-internal", metav1.GetOptions{})
			if err != nil {
				fmt.Printf("Failed getting config map skupper-internal; ignoring (%s)\n", err)
			}
			changed := cm.ResourceVersion != orig.ResourceVersion
			return changed, nil
		},
	)

	return err
}

func runHeyTesWithParameter(t *testing.T, cluster *base.ClusterContext, numOfWorkers string, durationOfTests string, jobName string, targetURL string) {

	waitJob := func(cc *base.ClusterContext, jobName string) {
		t.Helper()
		job, err := k8s.WaitForJob(cc.Namespace, cc.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
		_, _ = cc.KubectlExec("logs job/" + jobName)
		assert.Assert(t, err)
		k8s.AssertJob(t, job)
	}

	jobsClient := cluster.VanClient.KubeClient.BatchV1().Jobs(cluster.Namespace)

	// Set the parameters for Hey
	h1HeyBaseJob.Spec.Template.Spec.Containers[0].Args = []string{"-c", numOfWorkers, "-z", durationOfTests, targetURL}

	// Set new JobName
	h1HeyBaseJob.ObjectMeta.Name = jobName
	h1HeyBaseJob.Spec.Template.Name = jobName
	h1HeyBaseJob.Spec.Template.Spec.Containers[0].Name = jobName

	_, err := jobsClient.Create(context.TODO(), h1HeyBaseJob, metav1.CreateOptions{})
	assert.Assert(t, err)
	waitJob(cluster, jobName)

	_output, err := cluster.KubectlExec("logs job/" + jobName)
	assert.Assert(t, err)
	output := string(_output)

	// Check if tests passed
	retCode, errRegex := regexp.MatchString("\\[200\\].[[:digit:]]*.responses", output)
	assert.Assert(t, errRegex)
	assert.Assert(t, retCode)

	// Check if any tests did not pass
	retCode, errRegex = regexp.MatchString("\\[[3-5][0-9]+\\].[[:digit:]]*.responses", output)
	assert.Assert(t, errRegex)
	assert.Assert(t, !retCode)
}

// Create the test table for Hey and start tests
func runHeyTestTable(t *testing.T, jobCluster *base.ClusterContext) {

	testTable := []test{
		{
			name:            "h1hey5wrk30sec",
			doc:             "Send request using 5 concurrent workers during 30 seconds",
			cluster:         jobCluster,
			numOfWorkers:    "5",
			durationOfTests: "30s",
			jobName:         "h1hey5wrk30sec",
			targetURL:       "http://nginx1:8080",
		},
		{
			name:            "h1hey50wrk30sec",
			doc:             "Send request using 50 concurrent workers during 30 seconds",
			cluster:         jobCluster,
			numOfWorkers:    "50",
			durationOfTests: "30s",
			jobName:         "h1hey50wrk30sec",
			targetURL:       "http://nginx1:8080",
		},
		{
			name:            "h1hey5wrk60sec",
			doc:             "Send request using 5 concurrent workers during 60 seconds",
			cluster:         jobCluster,
			numOfWorkers:    "5",
			durationOfTests: "60s",
			jobName:         "h1hey5wrk60sec",
			targetURL:       "http://nginx1:8080",
		},
		{
			name:            "h1hey50wrk60sec",
			doc:             "Send request using 50 concurrent workers during 60 seconds",
			cluster:         jobCluster,
			numOfWorkers:    "50",
			durationOfTests: "60s",
			jobName:         "h1hey50wrk60sec",
			targetURL:       "http://nginx1:8080",
		},
	}

	// Iterate over test table
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			runHeyTesWithParameter(t, test.cluster, test.numOfWorkers, test.durationOfTests, test.jobName, test.targetURL)
		})
	}
}

func runTests(t *testing.T, r *base.ClusterTestRunnerBase) {
	pubCluster1, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nginx1")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nghttp2")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nghttp2tls")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nghttp2tcptls")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nghttp1tls")
	assert.Assert(t, err)

	runJob := func(cc *base.ClusterContext, jobName, testName string) {
		t.Helper()
		jobCmd := []string{"/app/http_test", "-test.run", testName}

		_, err = k8s.CreateTestJobWithSecret(cc.Namespace, cc.VanClient.KubeClient, jobName, jobCmd, types.ServiceClientSecret)
		assert.Assert(t, err)
	}

	waitJob := func(cc *base.ClusterContext, jobName string, t *testing.T) {
		t.Helper()
		job, err := k8s.WaitForJob(cc.Namespace, cc.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
		_, _ = cc.KubectlExec("logs job/" + jobName)
		assert.Assert(t, err)
		k8s.AssertJob(t, job)
	}

	// Send GET requests via HTTPD1
	t.Run("http1", func(t *testing.T) {
		runJob(pubCluster1, "http1", "TestHttpJob")
		waitJob(pubCluster1, "http1", t)
	})

	// Send GET requests via HTTPD2
	t.Run("http2", func(t *testing.T) {
		runJob(pubCluster1, "http2", "TestHttp2Job")
		waitJob(pubCluster1, "http2", t)
	})

	// Send GET requests via HTTP2 over TLS
	t.Run("http2tls", func(t *testing.T) {
		runJob(pubCluster1, "http2tls", "TestHttp2TlsJob")
		waitJob(pubCluster1, "http2tls", t)
	})

	// Send GET requests via HTTP2 over TLS on a service exposed with TCP protocol
	t.Run("http2tcptls", func(t *testing.T) {
		runJob(pubCluster1, "http2tcptls", "TestHttp2TcpTlsJob")
		waitJob(pubCluster1, "http2tcptls", t)
	})

	// Send GET requests via HTTP1 over TLS
	t.Run("http1tls", func(t *testing.T) {
		runJob(pubCluster1, "http1tls", "TestHttp1TlsJob")
		waitJob(pubCluster1, "http1tls", t)
	})

	// Send a huge load for HTTPD2
	t.Run("http2load", func(t *testing.T) {
		jobsClient := pubCluster1.VanClient.KubeClient.BatchV1().Jobs(pubCluster1.Namespace)
		_, err = jobsClient.Create(context.TODO(), h2loadJob, metav1.CreateOptions{})
		assert.Assert(t, err)
		waitJob(pubCluster1, "h2load", t)

		_output, err := pubCluster1.KubectlExec("logs job/" + "h2load")
		assert.Assert(t, err)
		output := string(_output)
		assert.Assert(t, strings.Contains(output, "1000 succeeded"), output)
	})

	// Call the test table for Hey tests
	runHeyTestTable(t, pubCluster1)
}

func setup(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	privateDeploymentsClient := prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace)

	createDeploymentInPrivateSite := func(dep *appsv1.Deployment) {
		t.Helper()
		fmt.Println("Creating nginx1 deployment...")
		result, err := privateDeploymentsClient.Create(ctx, dep, metav1.CreateOptions{})
		assert.Assert(t, err)

		fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	}

	// Create the deployment for HTTP
	createDeploymentInPrivateSite(nginxDep)

	// Create the configMap for index.html
	cmData := make(map[string]string)
	cmData["index.html"] = "<html><body>A simple HTTP Request &amp; Response Service.</body></html>"

	indexHTMLConfigMap := apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "index-html",
		},
		Data: cmData,
	}

	configMaps := prv1Cluster.VanClient.KubeClient.CoreV1().ConfigMaps(prv1Cluster.Namespace)
	_, err = configMaps.Create(ctx, &indexHTMLConfigMap, metav1.CreateOptions{})
	assert.Assert(t, err)

	// Create the deployment for HTTP2
	createDeploymentInPrivateSite(nghttp2Dep)

	// Create the deployment for HTTP2 with TLS enabled
	createDeploymentInPrivateSite(nghttp2TlsDep)

	// Create the deployment for HTTP2 with TLS enabled on a service exposed with TCP protocol
	createDeploymentInPrivateSite(nghttp2TcpTlsDep)

	// Create the deployment for HTTP1 with TLS enabled
	createDeploymentInPrivateSite(nghttp1TlsDep)

	fmt.Println("Creating and binding skupper service interfaces...")

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "nginx1", map[int]int{}, "")
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http2service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http2service, "deployment", "nghttp2", map[int]int{}, "")
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http2TlsService)
	assert.Assert(t, err)

	waitSecret(prv1Cluster, http2TlsService.TlsCredentials)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http2TlsService, "deployment", "nghttp2tls", map[int]int{}, "")
	assert.Assert(t, err)

	//update tls service with cert files
	err = updateDeployment(prv1Cluster, ctx, nghttp2TlsDepWithCertFiles)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http2TcpTlsService)
	assert.Assert(t, err)

	waitSecret(prv1Cluster, http2TcpTlsService.TlsCredentials)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http2TcpTlsService, "deployment", "nghttp2tcptls", map[int]int{}, "")
	assert.Assert(t, err)

	//update tls service with cert files
	err = updateDeployment(prv1Cluster, ctx, nghttp2tcpTlsDepWithCertFiles)
	assert.Assert(t, err)

	_, err = prv1Cluster.VanClient.KubeClient.CoreV1().ConfigMaps(prv1Cluster.Namespace).Create(context.TODO(), nghttp1TlsConfigMap, metav1.CreateOptions{})
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http1TlsService)
	assert.Assert(t, err)

	waitSecret(prv1Cluster, http1TlsService.TlsCredentials)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http1TlsService, "deployment", "nghttp1tls", map[int]int{}, "")
	assert.Assert(t, err)

	//update tls service with cert files
	err = updateDeployment(prv1Cluster, ctx, nghttp1TlsDepWithCertFiles)
	assert.Assert(t, err)

	http21service := types.ServiceInterface{
		Address:  "nghttp1",
		Protocol: "http",
		Ports:    []int{8443},
	}

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http21service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http21service, "deployment", "nghttp2", map[int]int{}, "")
	assert.Assert(t, err)

}

func Run(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	defer tearDown(ctx, r)
	defer func() {
		if t.Failed() {
			r.DumpTestInfo("TestHttp")
		}
	}()
	setup(ctx, t, r)
	runTests(t, r)
}

func tearDown(ctx context.Context, r *base.ClusterTestRunnerBase) {
	prv1Cluster, _ := r.GetPrivateContext(1)

	// Deleting Skupper services
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, service.Address)
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, http2service.Address)
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, http2TlsService.Address)
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, http2TcpTlsService.Address)
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, http1TlsService.Address)

	// Deleting deployments
	depCli := prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace)
	_ = depCli.Delete(ctx, nginxDep.Name, metav1.DeleteOptions{})
	_ = depCli.Delete(ctx, nghttp2Dep.Name, metav1.DeleteOptions{})
	_ = depCli.Delete(ctx, nghttp2TcpTlsDep.Name, metav1.DeleteOptions{})
	_ = depCli.Delete(ctx, nghttp2TlsDep.Name, metav1.DeleteOptions{})
	_ = depCli.Delete(ctx, nghttp1TlsDep.Name, metav1.DeleteOptions{})

	// Deleting configmaps
	configCli := prv1Cluster.VanClient.KubeClient.CoreV1().ConfigMaps(prv1Cluster.Namespace)
	_ = configCli.Delete(ctx, nghttp1TlsConfigMap.Name, metav1.DeleteOptions{})

	// Deleting secrets
	secretCli := prv1Cluster.VanClient.KubeClient.CoreV1().Secrets(prv1Cluster.Namespace)
	_ = secretCli.Delete(ctx, "skupper-tls-nghttp2tls", metav1.DeleteOptions{})
	_ = secretCli.Delete(ctx, "skupper-tls-nghttp1tls", metav1.DeleteOptions{})
	_ = secretCli.Delete(ctx, "skupper-tls-nghttp2tcptls", metav1.DeleteOptions{})

}
