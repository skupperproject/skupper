package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/certs"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"testing"
)

func TestCreateCertificateForService(t *testing.T) {

	// Mock VanClient
	const NAMESPACE = "test"
	// Used to define the test table
	type test struct {
		name          string
		targetService string
		siteId        string
		error         string
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
	kubeClient.Fake.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(k8stesting.GetAction).GetName()
		if name == "non-existent-ca" {
			return true, nil, fmt.Errorf("The CA for the site does not exists")
		}

		return false, nil, nil
	})

	kubeClient.Fake.PrependReactor("create", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(k8stesting.CreateAction).GetObject().(*corev1.Secret).Name

		if name == "existing-cert-error-svc" {
			return true, nil, fmt.Errorf("A certificate for that service already exists")
		}
		return false, nil, nil
	})

	// Creating the fake services
	svcNoPorts := newService("svc-no-ports")
	svcOnePort := newService("svc-one-port", 8080)
	siteId := "skupper-ca-site"
	caCert := certs.GenerateCASecret(siteId, siteId)
	kubeClient.CoreV1().Services(NAMESPACE).Create(svcNoPorts)
	kubeClient.CoreV1().Services(NAMESPACE).Create(svcOnePort)
	kubeClient.CoreV1().Secrets(NAMESPACE).Create(&caCert)

	testTable := []test{
		{"svc-no-ports", svcNoPorts.Name, siteId, ""},
		{"svc-one-port", svcOnePort.Name, siteId, ""},
		{"existing-cert-error-svc", "existing-cert-error-svc", siteId, "A certificate for that service already exists"},
		{"error-sit-ca", "error-sit-ca", "non-existent-ca", "The CA for the site does not exists"},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			cert, err := createCertificateForService(test.name, NAMESPACE, test.siteId, kubeClient)

			if cert != nil {
				assert.DeepEqual(t, test.name, cert.Name)
			}

			if test.error != "" {
				assert.Assert(t, err != nil)
				assert.Equal(t, test.error, err.Error())
			} else {
				assert.Assert(t, err == nil)
			}
		})
	}

}
