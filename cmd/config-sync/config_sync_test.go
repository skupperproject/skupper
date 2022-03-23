package main

import (
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"os"
	"testing"
)

const TEST_TLS_DIRECTORY = "./tmp/skupper-router/tls"

func TestSyncSecretsWithTlsEnabled(t *testing.T) {
	var err error

	stopCh := make(chan struct{})
	event.StartDefaultEventStore(stopCh)
	c := &ConfigSync{}
	kubeClient := fake.NewSimpleClientset()
	const NS = "fake"

	c.vanClient = &client.VanClient{
		KubeClient: kubeClient,
		Namespace:  NS,
	}

	c.agentPool = qdr.NewAgentPool("amqp://localhost:5672", nil)
	c.agentPool.Put(&qdr.Agent{})

	scenarios := []struct {
		doc              string
		before           *qdr.BridgeConfigDifference
		after            *qdr.BridgeConfigDifference
		sslProfileToSync string
		expected         string
	}{
		{
			doc:    "adding-http-connector-with-tls",
			before: &qdr.BridgeConfigDifference{},
			after: &qdr.BridgeConfigDifference{
				HttpConnectors: qdr.HttpEndpointDifference{
					Added: []qdr.HttpEndpoint{
						{
							Name:       "adservice",
							SslProfile: "skupper-service-client",
						},
					},
				},
			},
			sslProfileToSync: "skupper-service-client",
			expected:         "./tmp/skupper-router/tls/skupper-service-client",
		},
		{
			doc:    "adding-http-listener-with-tls",
			before: &qdr.BridgeConfigDifference{},
			after: &qdr.BridgeConfigDifference{
				HttpListeners: qdr.HttpEndpointDifference{
					Added: []qdr.HttpEndpoint{
						{
							Name:       "adservice",
							SslProfile: "skupper-tls-adservice",
						},
					},
				},
			},
			sslProfileToSync: "skupper-tls-adservice",
			expected:         "./tmp/skupper-router/tls/skupper-tls-adservice",
		},
		{
			doc: "removing-http-connector-with-tls",
			before: &qdr.BridgeConfigDifference{
				HttpConnectors: qdr.HttpEndpointDifference{
					Added: []qdr.HttpEndpoint{
						{
							Name:       "paymentservice",
							SslProfile: "skupper-service-client",
						},
					},
				},
				HttpListeners: qdr.HttpEndpointDifference{
					Added: []qdr.HttpEndpoint{
						{
							Name:       "adservice",
							SslProfile: "skupper-tls-adservice",
						},
					},
				},
			},
			after: &qdr.BridgeConfigDifference{
				HttpConnectors: qdr.HttpEndpointDifference{
					Deleted: []qdr.HttpEndpoint{
						{
							Name:       "paymentservice",
							SslProfile: "skupper-service-client",
						},
					},
				},
			},
			sslProfileToSync: "skupper-tls-adservice",
			expected:         "./tmp/skupper-router/tls/skupper-tls-adservice",
		},
	}

	for _, s := range scenarios {
		t.Run(s.doc, func(t *testing.T) {

			setUpKubernetesMock(c.vanClient)
			err = setUpTestingPath()
			assert.Assert(t, err)

			err = syncSecrets(c, s.before, TEST_TLS_DIRECTORY)
			assert.Assert(t, err)

			err = syncSecrets(c, s.after, TEST_TLS_DIRECTORY)
			assert.Assert(t, err)

			_, err := os.Stat(s.expected)
			missingDirectory := os.IsNotExist(err)
			assert.Assert(t, !missingDirectory, "missing directory: %v", s.expected)

			isDirEmpty, err := utils.IsDirEmpty(s.expected)
			assert.Assert(t, !isDirEmpty, "Directory is empty: %v", s.expected)

			_, err = os.Stat(s.expected + "/ca.crt")
			missingFile := os.IsNotExist(err)
			assert.Assert(t, !missingFile, "Missing ca.crt file")

			if s.sslProfileToSync != "skupper-service-client" {
				_, err = os.Stat(s.expected + "/tls.crt")
				missingFile = os.IsNotExist(err)
				assert.Assert(t, !missingFile, "Missing tls.crt file")

				_, err = os.Stat(s.expected + "/tls.key")
				missingFile = os.IsNotExist(err)
				assert.Assert(t, !missingFile, "Missing tls.key file")
			}

			os.RemoveAll(TEST_TLS_DIRECTORY)

		})
	}
}

func TestSyncSecretsWithoutTlsSupport(t *testing.T) {
	var err error

	stopCh := make(chan struct{})
	event.StartDefaultEventStore(stopCh)
	c := &ConfigSync{}
	kubeClient := fake.NewSimpleClientset()
	const NS = "fake"

	c.vanClient = &client.VanClient{
		KubeClient: kubeClient,
		Namespace:  NS,
	}

	scenarios := []struct {
		doc    string
		actual *qdr.BridgeConfigDifference
	}{

		{
			doc: "adding-http-listener-without-tls",
			actual: &qdr.BridgeConfigDifference{
				HttpListeners: qdr.HttpEndpointDifference{
					Added: []qdr.HttpEndpoint{
						{
							Name: "adservice",
						},
					},
				},
			},
		},
		{
			doc: "adding-tcp-listener",
			actual: &qdr.BridgeConfigDifference{
				TcpListeners: qdr.TcpEndpointDifference{
					Added: []qdr.TcpEndpoint{
						{
							Name: "adservice",
						},
					},
				},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.doc, func(t *testing.T) {

			err = setUpTestingPath()
			assert.Assert(t, err)

			err = syncSecrets(c, s.actual, TEST_TLS_DIRECTORY)
			assert.Assert(t, err)

			isDirEmpty, _ := utils.IsDirEmpty(TEST_TLS_DIRECTORY)
			assert.Assert(t, isDirEmpty, "Directory is not empty: %v", TEST_TLS_DIRECTORY)

			os.RemoveAll(TEST_TLS_DIRECTORY)

		})
	}
}

func TestCheckingSecretsWithTlsEnabled(t *testing.T) {
	var err error

	stopCh := make(chan struct{})
	event.StartDefaultEventStore(stopCh)

	kubeClient := fake.NewSimpleClientset()
	const NS = "fake"

	pool := qdr.NewAgentPool("amqp://localhost:5672", nil)
	pool.Put(&qdr.Agent{})

	c := &ConfigSync{
		vanClient: &client.VanClient{
			KubeClient: kubeClient,
			Namespace:  NS,
		},
		agentPool: pool,
	}

	scenarios := []struct {
		doc               string
		actual            *qdr.BridgeConfig
		sslProfileToCheck string
		expected          string
	}{
		{
			doc: "http-connector-with-tls",
			actual: &qdr.BridgeConfig{
				HttpConnectors: qdr.HttpEndpointMap{
					"adservice": qdr.HttpEndpoint{
						Name:       "adservice",
						SslProfile: "skupper-service-client",
					},
				},
			},
			sslProfileToCheck: "skupper-service-client",
			expected:          "./tmp/skupper-router/tls/skupper-service-client",
		},
		{
			doc: "http-listener-with-tls",
			actual: &qdr.BridgeConfig{
				HttpListeners: qdr.HttpEndpointMap{
					"adservice": qdr.HttpEndpoint{
						Name:       "adservice",
						SslProfile: "skupper-tls-adservice",
					},
				},
			},
			sslProfileToCheck: "skupper-tls-adservice",
			expected:          "./tmp/skupper-router/tls/skupper-tls-adservice",
		},
	}

	for _, s := range scenarios {
		t.Run(s.doc, func(t *testing.T) {

			setUpKubernetesMock(c.vanClient)
			err = setUpTestingPath()
			assert.Assert(t, err)

			err = c.checkSecrets(s.actual, TEST_TLS_DIRECTORY)
			assert.Assert(t, err)

			_, err := os.Stat(s.expected)
			missingDirectory := os.IsNotExist(err)
			assert.Assert(t, !missingDirectory, "missing directory: %v", s.expected)

			isDirEmpty, err := utils.IsDirEmpty(s.expected)
			assert.Assert(t, !isDirEmpty, "Directory is empty: %v", s.expected)

			_, err = os.Stat(s.expected + "/ca.crt")
			missingFile := os.IsNotExist(err)
			assert.Assert(t, !missingFile, "Missing ca.crt file")

			if s.sslProfileToCheck != "skupper-service-client" {
				_, err = os.Stat(s.expected + "/tls.crt")
				missingFile = os.IsNotExist(err)
				assert.Assert(t, !missingFile, "Missing tls.crt file")

				_, err = os.Stat(s.expected + "/tls.key")
				missingFile = os.IsNotExist(err)
				assert.Assert(t, !missingFile, "Missing tls.key file")
			}

			os.RemoveAll(TEST_TLS_DIRECTORY)

		})
	}
}

func setUpKubernetesMock(cli *client.VanClient) {
	if cli != nil {
		cli.KubeClient.(*fake.Clientset).Fake.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			secretName := action.(k8stesting.GetAction).GetName()

			if secretName == "skupper-service-client" {
				secret := v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: secretName,
					},
					Data: map[string][]byte{
						"ca.crt": []byte("ca.crt"),
					},
				}
				return true, &secret, nil
			} else {
				secret := v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: secretName,
					},
					Data: map[string][]byte{
						"tls.crt": []byte("tls.crt"),
						"tls.key": []byte("tls.key"),
						"ca.crt":  []byte("ca.crt"),
					},
				}
				return true, &secret, nil
			}
		})

	}
}

func setUpTestingPath() error {
	os.RemoveAll(TEST_TLS_DIRECTORY)

	err := os.MkdirAll(TEST_TLS_DIRECTORY, 0777)
	if err != nil {
		return err
	}

	return nil
}
