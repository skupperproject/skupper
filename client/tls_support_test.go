package client

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"os"
	"testing"
)

func TestCertAuthoritySiteCreate(t *testing.T) {
	testcases := []struct {
		doc           string
		namespace     string
		expectedError string
		skupperName   string
		siteUID       string
	}{
		{
			namespace:     "van-ca-site-create1",
			expectedError: "",
			doc:           "The certificate authority is created successfully.",
			skupperName:   "test-site",
			siteUID:       "dc9076e9",
		},
	}

	isCluster := *clusterRun

	for _, c := range testcases {
		_, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create the client
		var cli *VanClient
		var err error
		if !isCluster {
			lightRed := "\033[1;31m"
			resetColor := "\033[0m"
			t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
		}

		cli, err = NewClient(c.namespace, "", "")
		assert.Check(t, err, c.doc)

		_, err = kube.NewNamespace(c.namespace, cli.KubeClient)
		assert.Check(t, err, c.doc)
		defer func(name string, cli kubernetes.Interface) {
			err := kube.DeleteNamespace(name, cli)
			if err != nil {

			}
		}(c.namespace, cli.KubeClient)

		configureSiteAndCreateRouter(t, nil, cli, c.skupperName)

		assert.Check(t, err, c.doc)

		secret, err := cli.KubeClient.CoreV1().Secrets(c.namespace).Get(types.ServiceCaSecret, metav1.GetOptions{})

		assert.Check(t, secret != nil, "Secret "+types.ServiceCaSecret+" has not been created: %v", err)

		secret, err = cli.KubeClient.CoreV1().Secrets(c.namespace).Get(types.ServiceCaSecret, metav1.GetOptions{})

		assert.Check(t, secret != nil, "Secret "+types.ServiceClientSecret+" has not been created: %v", err)
	}
}

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

	// Create the client
	var cli, _ = newMockClient(NAMESPACE, "", "")

	// Mocking reacting to get a service and generate an error
	kubeClient := fake.NewSimpleClientset()
	cli.KubeClient = kubeClient
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

			_, err := cli.CreateSecretForService(test.targetService, test.targetService, secretName)

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
