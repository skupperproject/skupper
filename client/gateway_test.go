package client

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGatewayExportConfigAndGenerateBundle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-gateway-export-config-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	// isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, namespace)
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, namespace)
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-gateway-export-config-",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	gatewayName, observedError := cli.GatewayInit(ctx, "exportconfig", GatewayExportType, "")
	assert.Assert(t, observedError)
	assert.Equal(t, gatewayName, "exportconfig")

	// Here's where we will put the gateway download file.
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	// Create a few VAN Service Interfaces.
	echoService := types.ServiceInterface{
		Address:  "tcp-go-echo",
		Protocol: "tcp",
		Ports:    []int{9090},
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &echoService)
	assert.Assert(t, observedError)

	mongoService := types.ServiceInterface{
		Address:  "mongo-db",
		Protocol: "tcp",
		Ports:    []int{27017},
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &mongoService)
	assert.Assert(t, observedError)

	http1Service := types.ServiceInterface{
		Address:  "http1svc",
		Protocol: "http",
		Ports:    []int{10080},
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &http1Service)
	assert.Assert(t, observedError)

	http2Service := types.ServiceInterface{
		Address:  "http2svc",
		Protocol: "http2",
		Ports:    []int{10081},
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &http2Service)
	assert.Assert(t, observedError)

	// A few binds and forwards
	observedError = cli.GatewayBind(ctx, gatewayName, types.GatewayEndpoint{
		Host:    "localhost",
		Service: echoService,
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayBind(ctx, gatewayName, types.GatewayEndpoint{
		Host:    "localhost",
		Service: http1Service,
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, gatewayName, types.GatewayEndpoint{Service: mongoService}, true)
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, gatewayName, types.GatewayEndpoint{Service: http2Service}, true)
	assert.Assert(t, observedError)

	_, observedError = cli.GatewayExportConfig(ctx, "exportconfig", "myapp", testPath)
	assert.Assert(t, observedError)

	_, observedError = os.Stat(testPath + "myapp.yaml")
	//	file, observedError := os.Open(testPath + "myapp.yaml")
	//	defer file.Close()
	assert.Assert(t, observedError)

	_, observedError = cli.GatewayGenerateBundle(ctx, testPath+"myapp.yaml", testPath)
	assert.Assert(t, observedError)

	file, observedError := os.Open(testPath + "myapp.tar.gz")
	defer file.Close()
	assert.Assert(t, observedError)

	gzf, observedError := gzip.NewReader(file)
	assert.Assert(t, observedError)

	tarReader := tar.NewReader(gzf)

	files := []string{
		"qpid-dispatch-certs/conn1-profile/tls.crt",
		"qpid-dispatch-certs/conn1-profile/tls.key",
		"qpid-dispatch-certs/conn1-profile/ca.crt",
		"config/qdrouterd.json",
		"service/myapp.service",
		"launch.sh",
		"remove.sh",
		"expandvars.py",
	}

	i := 0
	for {
		header, observedError := tarReader.Next()

		if observedError == io.EOF {
			break
		}
		assert.Assert(t, observedError)
		assert.Equal(t, header.Name, files[i])

		i++
	}
	assert.Equal(t, i, len(files))

	// fire up a gateway with config
	gatewayName, observedError = cli.GatewayInit(ctx, "exportconfig2", GatewayExportType, testPath+"myapp.yaml")
	assert.Assert(t, observedError)
	assert.Equal(t, gatewayName, "exportconfig2")
}

func TestGatewayDownload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-gateway-download-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	// isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, namespace)
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, namespace)
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-gateway-download-",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	gatewayName, observedError := cli.GatewayInit(ctx, "download", GatewayExportType, "")
	assert.Assert(t, observedError)
	assert.Equal(t, gatewayName, "download")

	// Here's where we will put the gateway download file.
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	_, observedError = cli.GatewayDownload(ctx, "download", testPath)
	assert.Assert(t, observedError)

	file, observedError := os.Open(testPath + "download.tar.gz")
	defer file.Close()
	assert.Assert(t, observedError)

	gzf, observedError := gzip.NewReader(file)
	assert.Assert(t, observedError)

	tarReader := tar.NewReader(gzf)

	files := []string{
		"qpid-dispatch-certs/conn1-profile/tls.crt",
		"qpid-dispatch-certs/conn1-profile/tls.key",
		"qpid-dispatch-certs/conn1-profile/ca.crt",
		"config/qdrouterd.json",
		"service/download.service",
		"launch.sh",
		"remove.sh",
		"expandvars.py",
	}

	i := 0
	for {
		header, observedError := tarReader.Next()

		if observedError == io.EOF {
			break
		}
		assert.Assert(t, observedError)
		assert.Equal(t, header.Name, files[i])

		i++
	}
	assert.Equal(t, i, len(files))
}

func TestGatewayForward(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-gateway-forward-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	// isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-gateway-forward-")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-gateway-forward-")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-gateway-forward-",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	// setup listener to cause port collition
	l1, err := net.Listen("tcp", ":9090")
	assert.Assert(t, err)
	defer l1.Close()

	l2, err := net.Listen("tcp", ":9091")
	assert.Assert(t, err)
	defer l2.Close()

	// Create the VAN Service Interfaces.
	echoService := types.ServiceInterface{
		Address:  "tcp-go-echo",
		Protocol: "tcp",
		Ports:    []int{9090},
	}
	observedError := cli.ServiceInterfaceCreate(ctx, &echoService)
	assert.Assert(t, observedError)

	echoService2 := types.ServiceInterface{
		Address:  "tcp-go-echo2",
		Protocol: "tcp",
		Ports:    []int{9091},
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &echoService2)
	assert.Assert(t, observedError)

	mongoService := types.ServiceInterface{
		Address:  "mongo-db",
		Protocol: "tcp",
		Ports:    []int{27017},
	}

	observedError = cli.ServiceInterfaceCreate(ctx, &mongoService)
	assert.Assert(t, observedError)

	http1Service := types.ServiceInterface{
		Address:  "http1svc",
		Protocol: "http",
		Ports:    []int{10080},
	}

	observedError = cli.ServiceInterfaceCreate(ctx, &http1Service)
	assert.Assert(t, observedError)

	http2Service := types.ServiceInterface{
		Address:  "http2svc",
		Protocol: "http2",
		Ports:    []int{10081},
	}

	observedError = cli.ServiceInterfaceCreate(ctx, &http2Service)
	assert.Assert(t, observedError)

	gatewayName, observedError := cli.GatewayInit(ctx, namespace, GatewayExportType, "")
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, gatewayName, types.GatewayEndpoint{Service: echoService}, false)
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, gatewayName, types.GatewayEndpoint{Service: mongoService}, true)
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, gatewayName, types.GatewayEndpoint{Service: echoService2}, true)
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, gatewayName, types.GatewayEndpoint{Service: http1Service}, true)
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, gatewayName, types.GatewayEndpoint{Service: http2Service}, true)
	assert.Assert(t, observedError)

	gatewayInspect, observedError := cli.GatewayInspect(ctx, gatewayName)
	assert.Assert(t, observedError)
	assert.Equal(t, len(gatewayInspect.GatewayListeners), 5)
	assert.Equal(t, len(gatewayInspect.GatewayConnectors), 0)

	// Now undo
	observedError = cli.GatewayUnforward(ctx, gatewayName, types.GatewayEndpoint{Service: types.ServiceInterface{Address: "tcp-go-echo", Protocol: "tcp"}})
	assert.Assert(t, observedError)

	observedError = cli.GatewayUnforward(ctx, gatewayName, types.GatewayEndpoint{Service: types.ServiceInterface{Address: "http1svc", Protocol: "http"}})
	assert.Assert(t, observedError)

	observedError = cli.GatewayUnforward(ctx, gatewayName, types.GatewayEndpoint{Service: types.ServiceInterface{Address: "http2svc", Protocol: "http"}})
	assert.Assert(t, observedError)

	observedError = cli.GatewayRemove(ctx, gatewayName)
	assert.Assert(t, observedError)
}
func TestGatewayBind(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-gateway-bind-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	// isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-gateway-bind-")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-gateway-bind-")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-gateway-bind-",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	// Create the VAN Service Interfaces.
	service := types.ServiceInterface{
		Address:  "tcp-go-echo",
		Protocol: "tcp",
		Ports:    []int{9090},
	}
	observedError := cli.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, observedError)

	service.Address = "mongo-db"
	service.Ports = []int{27017}
	observedError = cli.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, observedError)

	http1Service := types.ServiceInterface{
		Address:  "http1svc",
		Protocol: "http",
		Ports:    []int{10080},
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &http1Service)
	assert.Assert(t, observedError)

	http2Service := types.ServiceInterface{
		Address:  "http2svc",
		Protocol: "http2",
		Ports:    []int{10081},
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &http2Service)
	assert.Assert(t, observedError)

	gatewayName, observedError := cli.GatewayInit(ctx, namespace, GatewayExportType, "")
	assert.Assert(t, observedError)

	observedError = cli.GatewayBind(ctx, gatewayName, types.GatewayEndpoint{
		Host: "localhost",
		Service: types.ServiceInterface{
			Protocol: "tcp",
			Ports:    []int{9090},
			Address:  "tcp-go-echo",
		},
	})

	assert.Assert(t, observedError)

	observedError = cli.GatewayBind(ctx, gatewayName, types.GatewayEndpoint{
		Host: "localhost",
		Service: types.ServiceInterface{
			Protocol: "tcp",
			Ports:    []int{27017},
			Address:  "mongo-db",
		},
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayBind(ctx, gatewayName, types.GatewayEndpoint{
		Host:    "localhost",
		Service: http1Service,
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayBind(ctx, gatewayName, types.GatewayEndpoint{
		Host:    "localhost",
		Service: http2Service,
	})
	assert.Assert(t, observedError)

	// Now undo
	observedError = cli.GatewayUnbind(ctx, gatewayName, types.GatewayEndpoint{Service: types.ServiceInterface{Address: "tcp-go-echo", Protocol: "tcp"}})
	assert.Assert(t, observedError)

	observedError = cli.GatewayUnbind(ctx, gatewayName, types.GatewayEndpoint{Service: types.ServiceInterface{Address: "http1svc", Protocol: "http"}})
	assert.Assert(t, observedError)

	observedError = cli.GatewayUnbind(ctx, gatewayName, types.GatewayEndpoint{Service: types.ServiceInterface{Address: "http2svc", Protocol: "http2"}})
	assert.Assert(t, observedError)

	gatewayInspect, observedError := cli.GatewayInspect(ctx, gatewayName)
	assert.Assert(t, observedError)
	assert.Equal(t, len(gatewayInspect.GatewayListeners), 0)
	assert.Equal(t, len(gatewayInspect.GatewayConnectors), 1)

	observedError = cli.GatewayRemove(ctx, gatewayName)
	assert.Assert(t, observedError)
}

func TestGatewayExpose(t *testing.T) {
	t.Skip("Skipping gateway expose test")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-gateway-expose-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	//isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-gateway-expose-")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-gateway-expose-")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-gateway-expose-",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	gatewayName, observedError := cli.GatewayExpose(ctx, namespace, GatewayExportType, types.GatewayEndpoint{
		Host: "localhost",
		Service: types.ServiceInterface{
			Protocol: "tcp",
			Ports:    []int{27017},
			Address:  "mongo-db",
		},
	})
	assert.Assert(t, observedError)

	gatewayInspect, observedError := cli.GatewayInspect(ctx, gatewayName)
	assert.Assert(t, observedError)
	assert.Equal(t, len(gatewayInspect.GatewayListeners), 0)
	assert.Equal(t, len(gatewayInspect.GatewayConnectors), 1)

	// Now undo
	observedError = cli.GatewayUnexpose(ctx, namespace, types.GatewayEndpoint{
		Host: "localhost",
		Service: types.ServiceInterface{
			Protocol: "tcp",
			Ports:    []int{27017},
			Address:  "mongo-db",
		},
	}, true)
	assert.Assert(t, observedError)
}

func TestGatewayInit(t *testing.T) {
	testcases := []struct {
		doc           string
		init          bool
		gwType        string
		initName      string
		actualName    string
		remove        bool
		removeName    string
		expectedError string
		url           string
	}{
		{
			init:          true,
			gwType:        GatewayExportType,
			initName:      "",
			actualName:    "",
			remove:        true,
			removeName:    "",
			expectedError: "",
			url:           "not active",
		},
		{
			init:          false,
			gwType:        GatewayExportType,
			initName:      "gateway1",
			actualName:    "gateway1",
			remove:        true,
			removeName:    "gateway1",
			expectedError: "onfigmaps \"skupper-gateway-gateway1\" not found",
			url:           "not active",
		},
		{
			init:          true,
			gwType:        GatewayExportType,
			initName:      "gateway2",
			actualName:    "gateway2",
			remove:        true,
			removeName:    "gateway2",
			expectedError: "",
			url:           "not active",
		},
		//		{
		//			init:          true,
		//			gwType:        GatewayDockerType,
		//			initName:      "gateway3",
		//			actualName:    "gateway3",
		//			remove:        true,
		//			removeName:    "gateway3",
		//			expectedError: "",
		//			url:           "amqp://127.0.0.1:5672",
		//		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-gateway-init-remove-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-gateway-init-remove")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-gateway-init-remove")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	_, observedError := cli.GatewayInit(ctx, namespace, GatewayExportType, "")
	assert.Check(t, strings.Contains(observedError.Error(), "Skupper not initialized"))

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-gateway-init-remove",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	// Init loop
	for _, tc := range testcases {
		if tc.init {
			if tc.actualName == "" {
				tc.actualName, _ = getUserDefaultGatewayName()
			}
			gatewayName, observedError := cli.GatewayInit(ctx, tc.initName, tc.gwType, "")
			assert.Assert(t, observedError)
			assert.Equal(t, gatewayName, tc.actualName)

			if tc.gwType != GatewayExportType {
				time.Sleep(time.Second * 1)
			}
			gatewayInspect, observedError := cli.GatewayInspect(ctx, gatewayName)
			assert.Assert(t, observedError)
			assert.Equal(t, gatewayInspect.GatewayName, tc.actualName)
			assert.Equal(t, gatewayInspect.GatewayUrl, tc.url)

			secret, observedError := cli.KubeClient.CoreV1().Secrets(namespace).Get(gatewayPrefix+gatewayName, metav1.GetOptions{})
			assert.Assert(t, observedError)
			ct, ok := secret.Labels[types.SkupperTypeQualifier]
			assert.Assert(t, ok)
			assert.Equal(t, ct, types.TypeGatewayToken)
		}
	}

	// Remove loop
	for _, tc := range testcases {
		if tc.remove {
			observedError = cli.GatewayRemove(ctx, tc.removeName)

			switch tc.expectedError {
			case "":
				assert.Check(t, observedError == nil || strings.Contains(observedError.Error(), "already defined"), "Test failure: An error was reported where none was expected. The error was |%s|.\n", observedError)
			default:
				if observedError == nil {
					assert.Check(t, observedError != nil, "Test failure: The expected error |%s| was not reported.\n", tc.expectedError)
				} else {
					assert.Check(t, strings.Contains(observedError.Error(), tc.expectedError), "Test failure: The reported error |%s| did not have the expected prefix |%s|.\n", observedError.Error(), tc.expectedError)
				}
			}
		}
	}

}
