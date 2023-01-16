package qdr

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/service"
	"github.com/skupperproject/skupper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"strings"
	"testing"

	"gotest.tools/assert"
)

func TestEnableTlsSupport(t *testing.T) {

	var tests = []struct {
		name          string
		tlsSupport    TlsServiceSupport
		tlsMocks      MockTls
		expectedError string
	}{
		{
			name: "Service does not need TLS support",
			tlsSupport: TlsServiceSupport{
				Address: "service1",
			},
		},
		{
			name: "Certificates should be generated by Skupper, the secret and the profile do not exist in the cluster",
			tlsSupport: TlsServiceSupport{
				Address:     "service2",
				Credentials: types.SkupperServiceCertPrefix + "service2",
			},
			tlsMocks: MockTls{
				GetSecretResult:     "error",
				GetConfigMapResult:  "ok",
				NewSecretResult:     "ok",
				AddSslProfileResult: "ok",
			},
		},
		{
			name: "Certificates are generated by Skupper, the secret already exists cluster",
			tlsSupport: TlsServiceSupport{
				Address:     "service3",
				Credentials: types.SkupperServiceCertPrefix + "service3",
			},
			tlsMocks: MockTls{
				GetSecretResult: "ok",
			},
		},
		{
			name: "Certificates are generated by Skupper, error getting the config map",
			tlsSupport: TlsServiceSupport{
				Address:     "service3",
				Credentials: types.SkupperServiceCertPrefix + "service3",
			},
			tlsMocks: MockTls{
				GetSecretResult:    "error",
				GetConfigMapResult: "error",
			},
			expectedError: "error getting the configmap",
		},
		{
			name: "Certificates are generated by Skupper, error creating the new secret",
			tlsSupport: TlsServiceSupport{
				Address:     "service",
				Credentials: types.SkupperServiceCertPrefix + "service",
			},
			tlsMocks: MockTls{
				GetSecretResult:    "error",
				GetConfigMapResult: "ok",
				NewSecretResult:    "error",
			},
			expectedError: "Failed to retrieve CA: secret skupper-service-ca do not exist",
		},
		{
			name: "Certificates are generated by Skupper, error adding ssl profile",
			tlsSupport: TlsServiceSupport{
				Address:     "service",
				Credentials: types.SkupperServiceCertPrefix + "service",
			},
			tlsMocks: MockTls{
				GetSecretResult:     "error",
				GetConfigMapResult:  "ok",
				NewSecretResult:     "ok",
				AddSslProfileResult: "error",
			},
			expectedError: "error adding the ssl profile",
		},
		{
			name: "Certificates are customised, but the secret with the CA does not exist in the cluster",
			tlsSupport: TlsServiceSupport{
				Address:       "service",
				Credentials:   "custom-credentials-service",
				CertAuthority: "custom-credentials-service-ca",
			},
			tlsMocks: MockTls{
				GetSecretResult:        "failsOnlySearchingByCA",
				ExistsSslProfileResult: "false",
				AddSslProfileResult:    "ok",
			},
			expectedError: "The secret custom-credentials-service-ca for address service is missing",
		},
		{
			name: "Certificates are customised and the credentials and CA secrets exist and the sslProfile does not need to be added",
			tlsSupport: TlsServiceSupport{
				Address:       "service",
				Credentials:   "custom-credentials-service",
				CertAuthority: "custom-credentials-service-ca",
			},
			tlsMocks: MockTls{
				GetSecretResult:        "ok",
				GetConfigMapResult:     "ok",
				ExistsSslProfileResult: "true",
			},
		},
		{
			name: "Certificates are customised and the credentials and CA secrets exist and the sslProfile needs to be added",
			tlsSupport: TlsServiceSupport{
				Address:       "service",
				Credentials:   "custom-credentials-service",
				CertAuthority: "custom-credentials-service-ca",
			},
			tlsMocks: MockTls{
				GetSecretResult:        "ok",
				GetConfigMapResult:     "ok",
				ExistsSslProfileResult: "false",
				AddSslProfileResult:    "ok",
			},
		},
		{
			name: "Fails when checking the sslProfile in the router config",
			tlsSupport: TlsServiceSupport{
				Address:       "service",
				Credentials:   "custom-credentials-service",
				CertAuthority: "custom-credentials-service-ca",
			},
			tlsMocks: MockTls{
				GetSecretResult:        "ok",
				GetConfigMapResult:     "ok",
				ExistsSslProfileResult: "error",
			},
			expectedError: "Error checking if if credentials exist in the router config",
		},
	}
	for _, test := range tests {
		kubeClient := fake.NewSimpleClientset()
		setUpKubernetesTlsMock(kubeClient, test.tlsMocks)
		tlsManager := TlsManager{kubeClient, "testing-enabling-tls"}

		fmt.Println(test.name)
		err := tlsManager.EnableTlsSupport(test.tlsSupport)
		if test.expectedError != "" {
			assert.Error(t, err, test.expectedError, test.name)
		}

	}
}

func TestDisableTlsSupport(t *testing.T) {

	var tests = []struct {
		name           string
		tlsCredentials string
		tlsMock        MockTls
		serviceList    []*types.ServiceInterface
		expectedError  string
	}{
		{
			name:           "Service does not need TLS support",
			tlsCredentials: "",
		},
		{
			name:           "Certificates are generated by Skupper, the secret and the profile should be deleted",
			tlsCredentials: types.SkupperServiceCertPrefix + "service2",
			tlsMock: MockTls{
				GetSecretResult:        "ok",
				RemoveSslProfileResult: "ok",
				DeleteSecretResult:     "ok",
			},
		},
		{
			name:           "Certificates are customised, the profile and secrets should not be deleted",
			tlsCredentials: "custom-service",
		},
		{
			name:           "It should not fail when trying to delete a secret that is not available",
			tlsCredentials: types.SkupperServiceCertPrefix + "service2",
			tlsMock: MockTls{
				GetSecretResult:        "error",
				RemoveSslProfileResult: "ok",
			},
		},
		{
			name:           "It should return and error if it was not possible to remove the sslProfile",
			tlsCredentials: types.SkupperServiceCertPrefix + "service2",
			tlsMock: MockTls{
				RemoveSslProfileResult: "error",
			},
			expectedError: "error removing sslprofile",
		},
		{
			name:           "It should return an error if deleting a secret has failed",
			tlsCredentials: types.SkupperServiceCertPrefix + "service2",
			tlsMock: MockTls{
				GetSecretResult:        "ok",
				RemoveSslProfileResult: "ok",
				DeleteSecretResult:     "error",
			},
			expectedError: "error deleting skupper-tls-service2",
		},
		{
			name:           "Certificates are generated by Skupper, but the sslProfile is used by another service",
			tlsCredentials: types.SkupperServiceCertPrefix + "service2",
			serviceList: []*types.ServiceInterface{
				{Address: "service2", TlsCredentials: types.SkupperServiceCertPrefix + "service2"},
				{Address: "unexpected-service", TlsCredentials: types.SkupperServiceCertPrefix + "service2"},
			},
			expectedError: "cannot remove the sslprofile because there is more than one service using it",
		},
	}
	for _, test := range tests {

		kubeClient := fake.NewSimpleClientset()
		setUpKubernetesTlsMock(kubeClient, test.tlsMock)
		tlsManager := TlsManager{kubeClient, "testing-disable-tls"}

		fmt.Println(test.name)
		err := tlsManager.DisableTlsSupport(test.tlsCredentials, test.serviceList)
		if test.expectedError != "" {
			assert.Error(t, err, test.expectedError, test.name)
		}

	}
}

func TestCheckBindingSecrets(t *testing.T) {

	var tests = []struct {
		name          string
		bindings      map[string]*service.ServiceBindings
		mockedSecrets []string
		expectedError string
	}{
		{
			name: "Service does not need TLS support",
			bindings: map[string]*service.ServiceBindings{
				"service1": {
					Address:          "service1",
					TlsCertAuthority: "",
					TlsCredentials:   "",
				},
			},
		},
		{
			name: "The services that need it have the required secrets in the cluster",
			bindings: map[string]*service.ServiceBindings{
				"service1": {
					Address:          "service1",
					TlsCertAuthority: "",
					TlsCredentials:   "",
				},
				"service2": {
					Address:          "service2",
					TlsCertAuthority: "skupper-tls-service2",
					TlsCredentials:   "skupper-tls-client",
				},
			},
			mockedSecrets: []string{"skupper-tls-service2", "skupper-tls-client"},
		},
		{
			name: "The secret with the server credentials is missing in the cluster",
			bindings: map[string]*service.ServiceBindings{
				"service1": {
					Address:          "service1",
					TlsCertAuthority: "",
					TlsCredentials:   "",
				},
				"service2": {
					Address:          "service2",
					TlsCertAuthority: "skupper-tls-service2",
					TlsCredentials:   "skupper-tls-client",
				},
			},
			mockedSecrets: []string{"skupper-tls-client"},
			expectedError: "SslProfile skupper-tls-service2 for service service2 does not exist in this cluster",
		},
		{
			name: "The secret that contains the CA is missing in the cluster",
			bindings: map[string]*service.ServiceBindings{
				"service1": {
					Address:          "service1",
					TlsCertAuthority: "",
					TlsCredentials:   "",
				},
				"service2": {
					Address:          "service2",
					TlsCertAuthority: "skupper-tls-service2",
					TlsCredentials:   "skupper-tls-client",
				},
			},
			mockedSecrets: []string{"skupper-tls-service2"},
			expectedError: "SslProfile skupper-tls-client for service service2 does not exist in this cluster",
		},
		{
			name: "There are no secrets in the cluster related to the bindings",
			bindings: map[string]*service.ServiceBindings{
				"service1": {
					Address:          "service1",
					TlsCertAuthority: "",
					TlsCredentials:   "",
				},
				"service2": {
					Address:          "service2",
					TlsCertAuthority: "skupper-tls-service2",
					TlsCredentials:   "skupper-tls-client",
				},
			},
			mockedSecrets: []string{},
			expectedError: "SslProfile skupper-tls-client for service service2 does not exist in this cluster",
		},
	}
	for _, test := range tests {

		kubeClient := fake.NewSimpleClientset()
		setUpKubernetesMock(kubeClient, test.mockedSecrets)

		fmt.Println(test.name)
		err := CheckBindingSecrets(test.bindings, "", kubeClient)
		if test.expectedError != "" {
			assert.Error(t, err, test.expectedError, test.name)
		}

	}
}

func setUpKubernetesMock(client *fake.Clientset, mockedSecrets []string) {
	client.Fake.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		secretName := action.(k8stesting.GetAction).GetName()

		if utils.StringSliceContains(mockedSecrets, secretName) {

			return true, nil, nil
		}
		return true, nil, fmt.Errorf("secret %s not found", secretName)
	})
}

type MockTls struct {
	GetSecretResult        string
	GetConfigMapResult     string
	NewSecretResult        string
	ExistsSslProfileResult string
	AddSslProfileResult    string
	RemoveSslProfileResult string
	DeleteSecretResult     string
}

func setUpKubernetesTlsMock(client *fake.Clientset, mocks MockTls) {
	setGetSecretMock(client, mocks)
	setGetConfigMapMock(client, mocks)
	setNewSecretMock(client, mocks)
	setDeleteSecretMock(client, mocks)
	setAddSslProfileMock(client, mocks)
	setExistSslProfileMock(client, mocks)
	setRemoveSslProfileMock(client, mocks)
}

func setGetSecretMock(client *fake.Clientset, mock MockTls) {
	switch mock.GetSecretResult {
	case "ok":
		client.Fake.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			secretName := action.(k8stesting.GetAction).GetName()
			secret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
			}
			return true, &secret, nil
		})
	case "error":
		client.Fake.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			secretName := action.(k8stesting.GetAction).GetName()
			return true, nil, fmt.Errorf("secret %s do not exist", secretName)
		})

	case "failsOnlySearchingByCA":
		client.Fake.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			secretName := action.(k8stesting.GetAction).GetName()
			if strings.Contains(secretName, "ca") {

				return true, nil, fmt.Errorf("secret %s do not exist", secretName)

			} else {
				secret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: secretName,
					},
				}
				return true, &secret, nil
			}
		})
	}
}

func setGetConfigMapMock(client *fake.Clientset, mock MockTls) {

	switch mock.GetConfigMapResult {
	case "ok":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(k8stesting.GetAction).GetName()
			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					UID:  kubetypes.UID(utils.RandomId(5)),
				},
			}
			return true, &configMap, nil
		})
	case "error":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {

			return true, nil, fmt.Errorf("error getting the configmap")
		})

	}
}

func setNewSecretMock(client *fake.Clientset, mock MockTls) {

	switch mock.NewSecretResult {
	case "ok":
		client.Fake.PrependReactor("create", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			secret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "newSecret",

					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "owner",
						},
					},
				},
			}

			return true, &secret, nil
		})
	case "error":
		client.Fake.PrependReactor("create", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("error creating the secret")
		})
	}
}
func setDeleteSecretMock(client *fake.Clientset, mock MockTls) {
	switch mock.DeleteSecretResult {
	case "ok":
		client.Fake.PrependReactor("delete", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, nil
		})
	case "error":
		client.Fake.PrependReactor("delete", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			secretName := action.(k8stesting.GetAction).GetName()
			return true, nil, fmt.Errorf("error deleting %s", secretName)
		})
	}
}

func setAddSslProfileMock(client *fake.Clientset, mock MockTls) {
	switch mock.AddSslProfileResult {
	case "ok":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(k8stesting.GetAction).GetName()
			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					UID:  kubetypes.UID(utils.RandomId(5)),
				},
			}
			return true, &configMap, nil
		})
	case "error":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("error adding the ssl profile")
		})
	}
}

func setExistSslProfileMock(client *fake.Clientset, mock MockTls) {
	switch mock.ExistsSslProfileResult {
	case "true":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(k8stesting.GetAction).GetName()
			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					UID:  kubetypes.UID(utils.RandomId(5)),
				},
			}
			return true, &configMap, nil
		})
	case "false":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(k8stesting.GetAction).GetName()
			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					UID:  kubetypes.UID(utils.RandomId(5)),
				},
			}
			return true, &configMap, nil
		})
	case "error":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {

			return true, nil, fmt.Errorf("error searching for sslProfile")
		})
	}
}

func setRemoveSslProfileMock(client *fake.Clientset, mock MockTls) {

	switch mock.RemoveSslProfileResult {
	case "ok":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			name := action.(k8stesting.GetAction).GetName()
			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					UID:  kubetypes.UID(utils.RandomId(5)),
				},
			}
			return true, &configMap, nil
		})
	case "error":
		client.Fake.PrependReactor("get", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {

			return true, nil, fmt.Errorf("error removing sslprofile")
		})
	}
}
