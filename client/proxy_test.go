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

func TestProxyDownload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-proxy-download-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	//isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-proxy-download-")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-proxy-download-")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-proxy-download-",
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

	proxyName, observedError := cli.ProxyInit(ctx, types.ProxyInitOptions{
		Name:       "download",
		StartProxy: false,
	})
	assert.Assert(t, observedError)
	assert.Equal(t, proxyName, "download")

	// Here's where we will put the proxy download file.
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	observedError = cli.ProxyDownload(ctx, "download", testPath)
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

func TestProxyForward(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-proxy-forward-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	//isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-proxy-forward-")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-proxy-forward-")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-proxy-forward-",
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

	proxyName, observedError := cli.ProxyInit(ctx, types.ProxyInitOptions{
		Name:       "",
		StartProxy: false,
	})
	assert.Assert(t, observedError)

	observedError = cli.ProxyForward(ctx, proxyName, false, &echoService)
	assert.Assert(t, observedError)

	observedError = cli.proxyStart(ctx, proxyName)
	assert.Assert(t, observedError)

	// Note: need delay for service to start up
	time.Sleep(time.Second * 1)

	observedError = cli.ProxyForward(ctx, proxyName, true, &mongoService)
	assert.Assert(t, observedError)

	observedError = cli.ProxyForward(ctx, proxyName, true, &echoService2)
	assert.Assert(t, observedError)

	proxyInspect, observedError := cli.ProxyInspect(ctx, proxyName)
	assert.Assert(t, observedError)
	assert.Equal(t, len(proxyInspect.TcpListeners), 3)
	assert.Equal(t, len(proxyInspect.TcpConnectors), 0)

	// Now undo
	observedError = cli.ProxyUnforward(ctx, proxyName, "tcp-go-echo")
	assert.Assert(t, observedError)

	observedError = cli.proxyStop(ctx, proxyName)
	assert.Assert(t, observedError)

	observedError = cli.ProxyRemove(ctx, proxyName)
	assert.Assert(t, observedError)
}
func TestProxyBind(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-proxy-bind-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	//isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-proxy-bind-")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-proxy-bind-")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-proxy-bind-",
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

	proxyName, observedError := cli.ProxyInit(ctx, types.ProxyInitOptions{
		Name:       "",
		StartProxy: false,
	})
	assert.Assert(t, observedError)

	egress := types.ProxyBindOptions{
		Protocol: "tcp",
		Host:     "localhost",
		Port:     "9090",
		Address:  "tcp-go-echo",
	}

	observedError = cli.ProxyBind(ctx, proxyName, egress)
	assert.Assert(t, observedError)

	observedError = cli.proxyStart(ctx, proxyName)
	assert.Assert(t, observedError)

	// Note: need delay for service to start up
	time.Sleep(time.Second * 1)

	egress.Address = "mongo-db"
	egress.Port = "27017"
	observedError = cli.ProxyBind(ctx, proxyName, egress)
	assert.Assert(t, observedError)

	// Now undo
	observedError = cli.ProxyUnbind(ctx, proxyName, "tcp-go-echo")
	assert.Assert(t, observedError)

	observedError = cli.proxyStop(ctx, proxyName)
	assert.Assert(t, observedError)

	observedError = cli.ProxyRemove(ctx, proxyName)
	assert.Assert(t, observedError)
}
func TestProxyExpose(t *testing.T) {
	testcases := []struct {
		doc             string
		expose          bool
		exposeProtocol  string
		exposeAddress   string
		exposeHost      string
		exposePort      string
		exposeName      string
		actualName      string
		unexpose        bool
		unexposeAddress string
		expectedError   string
		inspect         bool
		url             string
	}{
		{
			expose:          true,
			exposeProtocol:  "tcp",
			exposeAddress:   "tcp-go-echo",
			exposeHost:      "localhost",
			exposePort:      "9090",
			exposeName:      "",
			actualName:      "proxy1",
			unexpose:        true,
			unexposeAddress: "",
			expectedError:   "",
			inspect:         true,
			url:             "amqp://127.0.0.1:5672",
		},
		{
			expose:          true,
			exposeProtocol:  "tcp",
			exposeAddress:   "mongo-db",
			exposeHost:      "localhost",
			exposePort:      "27017",
			exposeName:      "",
			actualName:      "proxy2",
			unexpose:        true,
			unexposeAddress: "",
			expectedError:   "",
			inspect:         true,
			url:             "",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-proxy-expose-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	//isCluster := *clusterRun
	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-proxy-expose")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-proxy-expose")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-proxy-expose",
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

	// Expose loop
	for _, tc := range testcases {
		if tc.expose {
			exposeOptions := types.ProxyExposeOptions{
				ProxyName: tc.exposeName,
				Egress: types.ProxyBindOptions{
					Protocol: tc.exposeProtocol,
					Host:     tc.exposeHost,
					Port:     tc.exposePort,
					Address:  tc.exposeAddress,
				},
			}
			proxyName, observedError := cli.ProxyExpose(ctx, exposeOptions)
			assert.Assert(t, observedError)
			assert.Equal(t, proxyName, tc.actualName)
		}
	}

	// Note: need delay for service to start up
	time.Sleep(time.Second * 1)

	// Inspect loop
	for _, tc := range testcases {
		if tc.inspect {
			proxyInspect, observedError := cli.ProxyInspect(ctx, tc.actualName)
			assert.Assert(t, observedError)
			assert.Equal(t, proxyInspect.ProxyName, tc.actualName)
			if tc.url != "" {
				assert.Equal(t, proxyInspect.ProxyUrl, tc.url)
			}
		}
	}

	// Unexpose loop
	for _, tc := range testcases {
		if tc.unexpose {
			observedError := cli.ProxyUnexpose(ctx, tc.actualName, tc.exposeAddress)

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
func TestProxyInit(t *testing.T) {
	testcases := []struct {
		doc           string
		init          bool
		start         bool
		initName      string
		actualName    string
		remove        bool
		removeName    string
		expectedError string
		url           string
	}{
		{
			init:          true,
			start:         false,
			initName:      "",
			actualName:    "proxy1",
			remove:        true,
			removeName:    "",
			expectedError: "Unable to delete proxy definition, need proxy name",
			url:           "not active",
		},
		{
			init:          false,
			start:         false,
			initName:      "",
			actualName:    "proxy1",
			remove:        true,
			removeName:    "proxy1",
			expectedError: "",
			url:           "not active",
		},
		{
			init:          true,
			start:         false,
			initName:      "",
			actualName:    "proxy2",
			remove:        true,
			removeName:    "proxy2",
			expectedError: "",
			url:           "not active",
		},
		{
			init:          true,
			start:         false,
			initName:      "meow",
			actualName:    "meow",
			remove:        true,
			removeName:    "meow",
			expectedError: "",
			url:           "not active",
		},
		{
			init:          true,
			start:         true,
			initName:      "hippo",
			actualName:    "hippo",
			remove:        true,
			removeName:    "hippo",
			expectedError: "",
			url:           "amqp://127.0.0.1:5672",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error

	namespace := "test-proxy-init-remove-" + strings.ToLower(utils.RandomId(4))
	kubeContext := ""
	kubeConfigPath := ""

	if *clusterRun {
		cli, err = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli, err = newMockClient(namespace, kubeContext, kubeConfigPath)
	}
	assert.Check(t, err, "test-proxy-init-remove")
	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	assert.Check(t, err, "test-proxy-init-remove")
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	_, observedError := cli.ProxyInit(ctx, types.ProxyInitOptions{
		Name:       "",
		StartProxy: true,
	})
	assert.Check(t, strings.Contains(observedError.Error(), "Skupper not initialized"))

	// Create a router.
	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "test-proxy-init-remove",
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
			proxyName, observedError := cli.ProxyInit(ctx, types.ProxyInitOptions{
				Name:       tc.initName,
				StartProxy: tc.start,
			})
			assert.Assert(t, observedError)
			assert.Equal(t, proxyName, tc.actualName)

			if tc.start {
				time.Sleep(time.Second * 1)
			}
			proxyInspect, observedError := cli.ProxyInspect(ctx, proxyName)
			assert.Assert(t, observedError)
			assert.Equal(t, proxyInspect.ProxyName, tc.actualName)
			assert.Equal(t, proxyInspect.ProxyUrl, tc.url)
		}
	}

	// Remove loop
	for _, tc := range testcases {
		if tc.remove {
			observedError = cli.ProxyRemove(ctx, tc.removeName)

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
