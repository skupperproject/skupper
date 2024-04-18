package main

import (
	"context"
	"testing"

	oappsv1 "github.com/openshift/api/apps/v1"
	oappsfake "github.com/openshift/client-go/apps/clientset/versioned/fake"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"gotest.tools/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestInferTargetType(t *testing.T) {
	const NS = "mock-namespace"
	var cli *client.VanClient
	var tests = []struct {
		description  string
		expectedType string
		target       types.ServiceInterfaceTarget
	}{
		{
			description:  "deployment-expected",
			expectedType: "deployment",
			target: types.ServiceInterfaceTarget{
				Name:      "fake-deployment",
				Selector:  "app=fake-deployment",
				Namespace: NS,
			},
		},
		{
			description:  "statefulset-expected",
			expectedType: "statefulset",
			target: types.ServiceInterfaceTarget{
				Name:      "fake-statefulset",
				Selector:  "app=fake-statefulset",
				Namespace: NS,
			},
		},
		{
			description:  "deploymentconfig-expected",
			expectedType: "deploymentconfig",
			target: types.ServiceInterfaceTarget{
				Name:      "fake-deploymentconfig",
				Selector:  "app=fake-deploymentconfig",
				Namespace: NS,
			},
		},
		{
			description:  "annotated-service-expected",
			expectedType: "service",
			target: types.ServiceInterfaceTarget{
				Name:      "fake-app",
				Selector:  "app=fake-app",
				Namespace: NS,
			},
		},
		{
			description:  "direct-service-expected",
			expectedType: "service",
			target: types.ServiceInterfaceTarget{
				Name:      "fake-service",
				Service:   "fake-service",
				Namespace: NS,
			},
		},
		{
			description:  "undetermined-target-type",
			expectedType: "",
			target: types.ServiceInterfaceTarget{
				Name:      "absent-resource",
				Selector:  "app=absent-resource",
				Namespace: NS,
			},
		},
	}
	cli = &client.VanClient{
		Namespace:    NS,
		KubeClient:   fake.NewSimpleClientset(),
		OCAppsClient: oappsfake.NewSimpleClientset(),
	}
	assert.Assert(t, prepareFakeResources(cli, NS))

	pc := NewPolicyController(cli, event.NewDefaultEventLogger())
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert.Equal(t, tt.expectedType, pc.inferTargetType(tt.target, NS))
		})
	}
}

func prepareFakeResources(cli *client.VanClient, ns string) error {
	var err error
	// fake-deployment
	_, err = cli.KubeClient.AppsV1().Deployments(ns).Create(context.Background(),
		&v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-deployment",
				Namespace: ns,
				Labels: map[string]string{
					"app": "fake-deployment",
				},
			},
			Spec: v1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "fake-deployment",
					},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Ports: make([]corev1.ContainerPort, 8080),
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// fake-statefulset
	_, err = cli.KubeClient.AppsV1().StatefulSets(ns).Create(context.Background(),
		&v1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-statefulset",
				Namespace: ns,
				Labels: map[string]string{
					"app": "fake-statefulset",
				},
			},
			Spec: v1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "fake-statefulset",
					},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Ports: make([]corev1.ContainerPort, 8080),
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// fake-deploymentconfig
	_, err = cli.OCAppsClient.AppsV1().DeploymentConfigs(ns).Create(context.Background(),
		&oappsv1.DeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-deploymentconfig",
				Namespace: ns,
				Labels: map[string]string{
					"app": "fake-deploymentconfig",
				},
			},
			Spec: oappsv1.DeploymentConfigSpec{
				Selector: map[string]string{
					"app": "fake-deploymentconfig",
				},
				Template: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Ports: make([]corev1.ContainerPort, 8080),
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// fake-app (must prevail over deployment/fake-app)
	_, err = cli.KubeClient.CoreV1().Services(ns).Create(context.Background(),
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-app",
				Namespace: ns,
				Labels: map[string]string{
					"app": "fake-app",
				},
				Annotations: map[string]string{
					types.ProxyQualifier: "tcp",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "fake-app",
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// fake-app
	_, err = cli.KubeClient.AppsV1().Deployments(ns).Create(context.Background(),
		&v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-app",
				Namespace: ns,
				Labels: map[string]string{
					"app": "fake-app",
				},
			},
			Spec: v1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "fake-app",
					},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Ports: make([]corev1.ContainerPort, 8080),
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})
	return err
}
