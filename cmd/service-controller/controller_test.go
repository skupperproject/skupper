package main

import (
	"k8s.io/client-go/tools/record"
	"context"
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestCheckServiceFor(t *testing.T) {
	var err error
	kubeClient := fake.NewSimpleClientset()
	const NS = "fake"
	vanClient := &client.VanClient{
		KubeClient: kubeClient,
		Namespace:  NS,
		EventRecorder: kube.SkupperEventRecorder{
			EventRecorder: &record.FakeRecorder{},
			Disabled:      true,
		},
	}

	scenarios := []struct {
		doc      string
		actual   *corev1.Service
		desired  *types.ServiceInterface
		ports    []int
		expected *corev1.Service
	}{
		{
			doc: "same-ports",
			actual: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-same",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 8080}},
					},
					Selector: map[string]string{"app": "test"},
					Type:     corev1.ServiceTypeLoadBalancer,
				},
			},
			desired: &types.ServiceInterface{
				Protocol: "tcp",
				Address:  "test-svc-same",
				Labels: map[string]string{
					"app": "test",
				},
				Ports: []int{8080},
			},
			ports: []int{1024},
			expected: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-same",
					Annotations: map[string]string{
						types.OriginalTargetPortQualifier: "8080:8080",
						types.OriginalAssignedQualifier:   "8080:1024",
						types.OriginalSelectorQualifier:   "app=test",
					},
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 1024}},
					},
					Selector: map[string]string{
						"application":          types.TransportDeploymentName,
						"skupper.io/component": "router",
					},
				},
			},
		},
		{
			doc: "add-ports",
			actual: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-add",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 8080}},
					},
					Selector: map[string]string{"app": "test"},
					Type:     corev1.ServiceTypeLoadBalancer,
				},
			},
			desired: &types.ServiceInterface{
				Protocol: "tcp",
				Address:  "test-svc-add",
				Labels: map[string]string{
					"app": "test",
				},
				Ports: []int{8080, 9090},
			},
			ports: []int{1024, 1025},
			expected: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-add",
					Annotations: map[string]string{
						types.OriginalTargetPortQualifier: "8080:8080",
						types.OriginalAssignedQualifier:   "8080:1024,9090:1025",
						types.OriginalSelectorQualifier:   "app=test",
					},
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 1024}},
						{Name: "port9090", Port: 9090, TargetPort: intstr.IntOrString{IntVal: 1025}},
					},
					Selector: map[string]string{
						"application":          types.TransportDeploymentName,
						"skupper.io/component": "router",
					},
				},
			},
		},
		{
			doc: "del-ports",
			actual: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-del",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 8080}},
						{Name: "web2", Protocol: "tcp", Port: 8888, TargetPort: intstr.IntOrString{IntVal: 8888}},
					},
					Selector: map[string]string{"app": "test"},
					Type:     corev1.ServiceTypeLoadBalancer,
				},
			},
			desired: &types.ServiceInterface{
				Protocol: "tcp",
				Address:  "test-svc-del",
				Labels: map[string]string{
					"app": "test",
				},
				Ports: []int{8080},
			},
			ports: []int{1024},
			expected: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-del",
					Annotations: map[string]string{
						types.OriginalTargetPortQualifier: "8080:8080,8888:8888",
						types.OriginalAssignedQualifier:   "8080:1024",
						types.OriginalSelectorQualifier:   "app=test",
					},
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 1024}},
					},
					Selector: map[string]string{
						"application":          types.TransportDeploymentName,
						"skupper.io/component": "router",
					},
				},
			},
		},
		{
			doc: "add-del-ports",
			actual: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-add-del",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 8080}},
						{Name: "web2", Protocol: "tcp", Port: 8888, TargetPort: intstr.IntOrString{IntVal: 8888}},
					},
					Selector: map[string]string{"app": "test"},
					Type:     corev1.ServiceTypeLoadBalancer,
				},
			},
			desired: &types.ServiceInterface{
				Protocol: "tcp",
				Address:  "test-svc-add-del",
				Labels: map[string]string{
					"app": "test",
				},
				Ports: []int{8080, 9090},
			},
			ports: []int{1024, 1025},
			expected: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc-add-del",
					Annotations: map[string]string{
						types.OriginalTargetPortQualifier: "8080:8080,8888:8888",
						types.OriginalAssignedQualifier:   "8080:1024,9090:1025",
						types.OriginalSelectorQualifier:   "app=test",
					},
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "web", Protocol: "tcp", Port: 8080, TargetPort: intstr.IntOrString{IntVal: 1024}},
						{Name: "port9090", Port: 9090, TargetPort: intstr.IntOrString{IntVal: 1025}},
					},
					Selector: map[string]string{
						"application":          types.TransportDeploymentName,
						"skupper.io/component": "router",
					},
				},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.doc, func(t *testing.T) {
			_, err = kubeClient.CoreV1().Services(NS).Create(context.TODO(), s.actual, metav1.CreateOptions{})
			assert.Assert(t, err)
			stopCh := make(chan struct{})
			event.StartDefaultEventStore(stopCh)
			c, err := NewController(vanClient, "foo", nil, true)
			assert.Assert(t, err)
			go c.svcInformer.Run(stopCh)
			cache.WaitForCacheSync(stopCh, c.svcInformer.HasSynced)
			defer close(stopCh)

			err = c.realiseServiceBindings(*s.desired, s.ports)
			assert.Assert(t, err)

			// validating expected service
			svc, err := kubeClient.CoreV1().Services(NS).Get(context.TODO(), s.expected.Name, metav1.GetOptions{})
			assert.Assert(t, err)

			// Comparing services
			assert.Equal(t, len(svc.Spec.Ports), len(s.expected.Spec.Ports))
			for _, expPort := range s.expected.Spec.Ports {
				curPort := kube.GetServicePort(svc, int(expPort.Port))
				if expPort.Protocol == "" {
					expPort.Protocol = "TCP"
				}
				assert.Assert(t, reflect.DeepEqual(expPort, *curPort), "expected: %v - got: %v", expPort, curPort)
			}
			assert.Assert(t, reflect.DeepEqual(svc.Spec.Selector, s.expected.Spec.Selector), "expected: %v - got: %v", s.expected.Spec.Selector, svc.Spec.Selector)
			assert.Assert(t, reflect.DeepEqual(svc.ObjectMeta.Labels, s.expected.ObjectMeta.Labels), "expected: %v - got: %v", s.expected.ObjectMeta.Labels, svc.ObjectMeta.Labels)
			assert.Equal(t, len(svc.ObjectMeta.Annotations), len(s.expected.ObjectMeta.Annotations))
			for expAnnotation, expValue := range s.expected.ObjectMeta.Annotations {
				curValue, found := svc.ObjectMeta.Annotations[expAnnotation]
				assert.Assert(t, found)
				switch expAnnotation {
				case types.OriginalTargetPortQualifier:
					fallthrough
				case types.OriginalAssignedQualifier:
					curPortMap := kube.PortLabelStrToMap(curValue)
					expPortMap := kube.PortLabelStrToMap(expValue)
					assert.DeepEqual(t, curPortMap, expPortMap)
				case types.OriginalSelectorQualifier:
					curSelectorMap := utils.LabelToMap(curValue)
					expSelectorMap := utils.LabelToMap(expValue)
					assert.DeepEqual(t, curSelectorMap, expSelectorMap)
				}
			}
		})
	}
}
