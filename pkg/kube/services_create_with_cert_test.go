package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"os"
	"testing"
)

func TestCreateCertificateForService(t *testing.T) {

	// Mock VanClient
	const NAMESPACE = "test"
	const SITE_ID = "12345"

	// Used to define the test table
	type test struct {
		name          string
		targetService string
		address       string
		siteId        string
		error         string
		secret        string
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
		if name == "skupper-site-error-site-ca" {
			return true, nil, fmt.Errorf("The CA for the site does not exists")
		}

		return false, nil, nil
	})

	kubeClient.Fake.PrependReactor("create", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(k8stesting.CreateAction).GetObject().(*corev1.Secret).Name

		if name == "skupper-existing-cert-error-svc" {
			return true, nil, fmt.Errorf("A certificate for that service already exists")
		}
		return false, nil, nil
	})

	// Creating the fake services
	svcNoPorts := newService("svc-no-ports")
	svcOnePort := newService("svc-one-port", 8080)
	kubeClient.CoreV1().Services(NAMESPACE).Create(svcNoPorts)
	kubeClient.CoreV1().Services(NAMESPACE).Create(svcOnePort)

	testTable := []test{
		{"skupper-svc-no-ports", svcNoPorts.Name, "svc-no-ports", SITE_ID, "", "skupper-service-ca"},
		{"skupper-svc-one-port", svcOnePort.Name, "svc-one-port", SITE_ID, "", "skupper-service-ca"},
		{"skupper-existing-cert-error-svc", "existing-cert-error-svc", "existing-cert-error-svc", SITE_ID, "A certificate for that service already exists", "skupper-service-ca"},
		{"skupper-error-site-ca", "error-site-ca", "error-site-ca", "error-site-ca", "secrets \"skupper-service-ca\" not found", "skupper-site-error-site-ca"},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			caCert := certs.GenerateCASecret(test.secret, test.secret)
			os.Setenv("SKUPPER_SITE_ID", test.siteId)
			kubeClient.CoreV1().Secrets(NAMESPACE).Create(&caCert)
			secretName := types.SkupperServiceCertPrefix + test.targetService

			_, err := CreateSecretsForService(test.targetService, NAMESPACE, test.targetService, secretName, kubeClient)

			if err == nil {
				serviceSecret, err := kubeClient.CoreV1().Secrets(NAMESPACE).Get(types.SkupperServiceCertPrefix+test.targetService, metav1.GetOptions{})

				assert.Assert(t, err == nil)

				if serviceSecret != nil {
					assert.Equal(t, test.name, serviceSecret.Name)
				}
			}

			if test.error != "" {
				assert.Assert(t, err != nil)
				assert.Equal(t, test.error, err.Error())
			} else {
				assert.Assert(t, err == nil)
			}

			os.Remove("SKUPPER_SITE_ID")
			kubeClient.CoreV1().Secrets(NAMESPACE).Delete(caCert.Name, &metav1.DeleteOptions{})
		})
	}

}
