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
)

func TestGatewayDownload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-gateway-download-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	//isCluster := *clusterRun
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

	gatewayName, observedError := cli.GatewayInit(ctx, types.GatewayInitOptions{
		Name:         "download",
		DownloadOnly: true,
	})
	assert.Assert(t, observedError)
	assert.Equal(t, gatewayName, "download")

	// Here's where we will put the gateway download file.
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	observedError = cli.GatewayDownload(ctx, "download", testPath)
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

	//isCluster := *clusterRun
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
		Port:     9090,
	}
	observedError := cli.ServiceInterfaceCreate(ctx, &echoService)
	assert.Assert(t, observedError)

	echoService2 := types.ServiceInterface{
		Address:  "tcp-go-echo2",
		Protocol: "tcp",
		Port:     9091,
	}
	observedError = cli.ServiceInterfaceCreate(ctx, &echoService2)
	assert.Assert(t, observedError)

	mongoService := types.ServiceInterface{
		Address:  "mongo-db",
		Protocol: "tcp",
		Port:     27017,
	}

	observedError = cli.ServiceInterfaceCreate(ctx, &mongoService)
	assert.Assert(t, observedError)

	gatewayName, observedError := cli.GatewayInit(ctx, types.GatewayInitOptions{
		Name:         "",
		DownloadOnly: true,
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, types.GatewayForwardOptions{
		GatewayName: gatewayName,
		Loopback:    false,
		Service:     echoService,
	})
	assert.Assert(t, observedError)

	//	observedError = cli.gatewayStart(ctx, gatewayName)
	//	assert.Assert(t, observedError)

	// Note: need delay for service to start up
	//	time.Sleep(time.Second * 1)

	observedError = cli.GatewayForward(ctx, types.GatewayForwardOptions{
		GatewayName: gatewayName,
		Loopback:    true,
		Service:     mongoService,
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayForward(ctx, types.GatewayForwardOptions{
		GatewayName: gatewayName,
		Loopback:    true,
		Service:     echoService2,
	})
	assert.Assert(t, observedError)

	gatewayInspect, observedError := cli.GatewayInspect(ctx, gatewayName)
	assert.Assert(t, observedError)
	assert.Equal(t, len(gatewayInspect.TcpListeners), 3)
	assert.Equal(t, len(gatewayInspect.TcpConnectors), 0)

	// Now undo
	observedError = cli.GatewayUnforward(ctx, gatewayName, "tcp-go-echo")
	assert.Assert(t, observedError)

	//	observedError = cli.gatewayStop(ctx, gatewayName)
	//	assert.Assert(t, observedError)

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

	//isCluster := *clusterRun
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
		Port:     9090,
	}
	observedError := cli.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, observedError)

	service.Address = "mongo-db"
	service.Port = 27017
	observedError = cli.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, observedError)

	gatewayName, observedError := cli.GatewayInit(ctx, types.GatewayInitOptions{
		Name:         "",
		DownloadOnly: true,
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayBind(ctx, types.GatewayBindOptions{
		GatewayName: gatewayName,
		Protocol:    "tcp",
		Host:        "localhost",
		Port:        "9090",
		Address:     "tcp-go-echo",
	})
	assert.Assert(t, observedError)

	observedError = cli.GatewayBind(ctx, types.GatewayBindOptions{
		GatewayName: gatewayName,
		Protocol:    "tcp",
		Host:        "localhost",
		Port:        "27017",
		Address:     "mongo-db",
	})
	assert.Assert(t, observedError)

	// Now undo
	observedError = cli.GatewayUnbind(ctx, types.GatewayUnbindOptions{
		GatewayName: gatewayName,
		Address:     "tcp-go-echo",
	})
	assert.Assert(t, observedError)

	gatewayInspect, observedError := cli.GatewayInspect(ctx, gatewayName)
	assert.Assert(t, observedError)
	assert.Equal(t, len(gatewayInspect.TcpListeners), 0)
	assert.Equal(t, len(gatewayInspect.TcpConnectors), 1)

	observedError = cli.GatewayRemove(ctx, gatewayName)
	assert.Assert(t, observedError)
}
func TestGatewayInit(t *testing.T) {
	testcases := []struct {
		doc           string
		init          bool
		downloadOnly  bool
		initName      string
		actualName    string
		remove        bool
		removeName    string
		expectedError string
		url           string
	}{
		{
			init:          true,
			downloadOnly:  true,
			initName:      "",
			actualName:    "",
			remove:        true,
			removeName:    "",
			expectedError: "",
			url:           "not active",
		},
		{
			init:          false,
			downloadOnly:  true,
			initName:      "gateway1",
			actualName:    "gateway1",
			remove:        true,
			removeName:    "gateway1",
			expectedError: "",
			url:           "not active",
		},
		{
			init:          true,
			downloadOnly:  true,
			initName:      "gateway2",
			actualName:    "gateway2",
			remove:        true,
			removeName:    "gateway2",
			expectedError: "",
			url:           "not active",
		},
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

	_, observedError := cli.GatewayInit(ctx, types.GatewayInitOptions{
		Name:         "",
		DownloadOnly: false,
	})
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
			gatewayName, observedError := cli.GatewayInit(ctx, types.GatewayInitOptions{
				Name:         tc.initName,
				DownloadOnly: tc.downloadOnly,
			})
			assert.Assert(t, observedError)
			assert.Equal(t, gatewayName, tc.actualName)

			if !tc.downloadOnly {
				time.Sleep(time.Second * 1)
			}
			gatewayInspect, observedError := cli.GatewayInspect(ctx, gatewayName)
			assert.Assert(t, observedError)
			assert.Equal(t, gatewayInspect.GatewayName, tc.actualName)
			assert.Equal(t, gatewayInspect.GatewayUrl, tc.url)
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
