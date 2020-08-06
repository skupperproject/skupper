package kube

import (
	"fmt"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"testing"
)

func TestGetPortForServiceTarget(t *testing.T) {

	// Mock VanClient
	const NS = "test"
	// Used to define the test table
	type test struct {
		name          string
		targetService string
		error         string
		expected      int
	}

	// Helper functions used to compose test table
	newService := func(name string, ports ...int) *corev1.Service {
		// Only add ports when at least one has been provided
		var servicePorts []corev1.ServicePort
		if len(ports) > 0 {
			for i, port := range ports {
				servicePorts = append(servicePorts, corev1.ServicePort{
					Name: fmt.Sprintf("port%d", i),
					Port: int32(port),
				})
			}
		}

		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: corev1.ServiceSpec{
				Ports: servicePorts,
			},
		}
	}

	// Mocking reacting to get a service and generate an error
	kubeClient := fake.NewSimpleClientset()
	kubeClient.Fake.PrependReactor("get", "services", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(k8stesting.GetAction).GetName()
		if name == "error-svc" {
			return true, nil, fmt.Errorf("fake error has occurred")
		}
		return false, nil, nil
	})

	// Creating the fake services
	svcNoPorts := newService("svc-no-ports")
	svcOnePort := newService("svc-one-port", 8080)
	svcDotOnePort := newService("svc-one-port.test", 8080)
	svcThreePorts := newService("svc-three-ports", 8080, 8081, 8082)
	kubeClient.CoreV1().Services(NS).Create(svcNoPorts)
	kubeClient.CoreV1().Services(NS).Create(svcOnePort)
	kubeClient.CoreV1().Services(NS).Create(svcDotOnePort)
	kubeClient.CoreV1().Services(NS).Create(svcThreePorts)

	testTable := []test{
		{"svc-no-ports", svcNoPorts.Name, "", 0},
		{"svc-one-port", svcOnePort.Name, "", 8080},
		{"svc-dot-one-port", svcDotOnePort.Name, "", 8080},
		{"svc-three-ports", svcThreePorts.Name, "", 8080},
		{"invalid-svc", "invalid", "", 0},
		{"error-svc", "error-svc", "fake error has occurred", 0},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			port, err := GetPortForServiceTarget(test.targetService, NS, kubeClient)
			assert.Equal(t, test.expected, port)
			if test.error != "" {
				assert.Assert(t, err != nil)
				assert.Equal(t, test.error, err.Error())
			} else {
				assert.Assert(t, err == nil)
			}
		})
	}
}
