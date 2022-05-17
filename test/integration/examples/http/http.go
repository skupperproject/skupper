package http

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
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
	Address:        "nghttp2tls",
	Protocol:       "http2",
	Ports:          []int{8443},
	EnableTls:      true,
	TlsCredentials: "skupper-tls-nghttp2tls",
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
						Image:           "nginxinc/nginx-unprivileged:stable-alpine",
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
						Image:           "docker.io/svagi/nghttp2",
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
						Image:           "docker.io/svagi/nghttp2",
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
						Image:           "docker.io/svagi/nghttp2",
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
						Image: "docker.io/svagi/nghttp2",
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

func runHeyTesWithParameter(t *testing.T, cluster *base.ClusterContext, numOfWorkers string, durationOfTests string, jobName string, targetURL string) {

	waitJob := func(cc *base.ClusterContext, jobName string) {
		t.Helper()
		job, err := k8s.WaitForJob(cc.Namespace, cc.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
		_, _ = cc.KubectlExec("logs job/" + jobName)
		if err != nil {
			cc.DumpTestInfo(jobName)
		}
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

	_, err := jobsClient.Create(h1HeyBaseJob)
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

func runTests(t *testing.T, r base.ClusterTestRunner) {
	pubCluster1, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nginx1")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nghttp2")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "nghttp2tls")
	assert.Assert(t, err)

	runJob := func(cc *base.ClusterContext, jobName, testName string) {
		t.Helper()
		jobCmd := []string{"/app/http_test", "-test.run", testName}

		_, err = k8s.CreateTestJobWithSecret(cc.Namespace, cc.VanClient.KubeClient, jobName, jobCmd, types.ServiceClientSecret)
		assert.Assert(t, err)
	}

	waitJob := func(cc *base.ClusterContext, jobName string) {
		t.Helper()
		job, err := k8s.WaitForJob(cc.Namespace, cc.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
		_, _ = cc.KubectlExec("logs job/" + jobName)
		if err != nil {
			cc.DumpTestInfo(jobName)
		}
		assert.Assert(t, err)
		k8s.AssertJob(t, job)
	}

	// Send GET requests via HTTPD1
	t.Run("http1", func(t *testing.T) {
		runJob(pubCluster1, "http1", "TestHttpJob")
		waitJob(pubCluster1, "http1")
	})

	// Send GET requests via HTTPD2
	t.Run("http2", func(t *testing.T) {
		runJob(pubCluster1, "http2", "TestHttp2Job")
		waitJob(pubCluster1, "http2")
	})

	// Send GET requests via HTTPD2
	t.Run("http2tls", func(t *testing.T) {
		runJob(pubCluster1, "http2tls", "TestHttp2TlsJob")
		waitJob(pubCluster1, "http2tls")
	})

	// Send a huge load for HTTPD2
	t.Run("http2load", func(t *testing.T) {
		jobsClient := pubCluster1.VanClient.KubeClient.BatchV1().Jobs(pubCluster1.Namespace)
		_, err = jobsClient.Create(h2loadJob)
		assert.Assert(t, err)
		waitJob(pubCluster1, "h2load")

		_output, err := pubCluster1.KubectlExec("logs job/" + "h2load")
		assert.Assert(t, err)
		output := string(_output)
		assert.Assert(t, strings.Contains(output, "1000 succeeded"), output)
	})

	// Call the test table for Hey tests
	runHeyTestTable(t, pubCluster1)
}

func setup(ctx context.Context, t *testing.T, r base.ClusterTestRunner) {
	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	privateDeploymentsClient := prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace)

	createDeploymentInPrivateSite := func(dep *appsv1.Deployment) {
		t.Helper()
		fmt.Println("Creating nginx1 deployment...")
		result, err := privateDeploymentsClient.Create(dep)
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
	_, err = configMaps.Create(&indexHTMLConfigMap)
	assert.Assert(t, err)

	// Create the deployment for HTTP2
	createDeploymentInPrivateSite(nghttp2Dep)

	// Create the deployment for HTTP2 with TLS enabled
	createDeploymentInPrivateSite(nghttp2TlsDep)

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "nginx1", "http", map[int]int{})
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http2service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http2service, "deployment", "nghttp2", "http2", map[int]int{})
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http2TlsService)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http2TlsService, "deployment", "nghttp2tls", "http2", map[int]int{})
	assert.Assert(t, err)

	//update tls service with cert files
	_, err = prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace).Update(nghttp2TlsDepWithCertFiles)
	assert.Assert(t, err)

	http21service := types.ServiceInterface{
		Address:  "nghttp1",
		Protocol: "http",
		Ports:    []int{8443},
	}

	err = prv1Cluster.VanClient.ServiceInterfaceCreate(ctx, &http21service)
	assert.Assert(t, err)

	err = prv1Cluster.VanClient.ServiceInterfaceBind(ctx, &http21service, "deployment", "nghttp2", "http", map[int]int{})
	assert.Assert(t, err)

}

func Run(ctx context.Context, t *testing.T, r base.ClusterTestRunner) {
	defer tearDown(ctx, r)
	setup(ctx, t, r)
	runTests(t, r)
}

func tearDown(ctx context.Context, r base.ClusterTestRunner) {
	prv1Cluster, _ := r.GetPrivateContext(1)

	// Deleting Skupper services
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, service.Address)
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, http2service.Address)
	_ = prv1Cluster.VanClient.ServiceInterfaceRemove(ctx, http2TlsService.Address)

	// Deleting deployments
	depCli := prv1Cluster.VanClient.KubeClient.AppsV1().Deployments(prv1Cluster.Namespace)
	_ = depCli.Delete(nginxDep.Name, &metav1.DeleteOptions{})
	_ = depCli.Delete(nghttp2Dep.Name, &metav1.DeleteOptions{})
}
