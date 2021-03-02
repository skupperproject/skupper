package client

import (
	"context"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"

	"gotest.tools/assert"
)

func TestServiceInterfaceInspect(t *testing.T) {
	testcases := []struct {
		namespace             string
		doc                   string
		addr                  string
		proto                 string
		port                  int
		init                  bool
		expectedCreationError string
	}{
		{
			namespace:             "vsii-1",
			addr:                  "vsii-1-addr",
			proto:                 "tcp",
			port:                  5672,
			init:                  true,
			expectedCreationError: "",
		},
		{
			namespace:             "vsii-2",
			addr:                  "vsii-2-addr",
			proto:                 "tcp",
			port:                  5672,
			init:                  false,
			expectedCreationError: "Skupper not initialised",
		},
	}
	for _, testcase := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Run in a real cluster, or in a mock environment.
		var cli *VanClient
		var err error
		isCluster := *clusterRun
		if isCluster {
			cli, err = NewClient(testcase.namespace, "", "")
		} else {
			cli, err = newMockClient(testcase.namespace, "", "")
		}
		assert.Check(t, err, testcase.namespace)

		_, err = kube.NewNamespace(testcase.namespace, cli.KubeClient)
		assert.Check(t, err, "%s: Namespace creation failed.", testcase.namespace)
		defer kube.DeleteNamespace(testcase.namespace, cli.KubeClient)

		// Create a skupper router -- or don't if the test
		// wants a creation error.
		if testcase.init {
			err = cli.RouterCreate(ctx, types.SiteConfig{
				Spec: types.SiteConfigSpec{
					SkupperName:       testcase.namespace,
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
			assert.Check(t, err, "%s: Unable to create VAN router", testcase.namespace)
		}

		// Create the ServiceInterface.
		service := types.ServiceInterface{
			Address:  testcase.addr,
			Protocol: testcase.proto,
			Port:     testcase.port,
		}
		err = cli.ServiceInterfaceCreate(ctx, &service)

		// If initialization was not done, we should see an error.
		// In this case, don't try to check the Service Interface --
		// it isn't there.
		if testcase.expectedCreationError != "" {
			assert.Check(t,
				err != nil && strings.Contains(err.Error(), testcase.expectedCreationError),
				"\n\nTest %s failure: The expected error |%s| was not reported.\n",
				testcase.namespace,
				testcase.expectedCreationError)
		} else {
			assert.Check(t, err, "\n\nTest %s failure: Creation failed.\n", testcase.namespace)

			// When we inspect the ServiceInterface, make sure that the
			// expected values have been set.
			serviceInterface, err := cli.ServiceInterfaceInspect(ctx, testcase.addr)
			assert.Check(t, err, "Inspection failed.")

			assert.Equal(t, testcase.addr, serviceInterface.Address,
				"\n\nTest %s failure: Address was |%s| but should be |%s|.\n",
				testcase.namespace,
				serviceInterface.Address,
				testcase.addr)
			assert.Equal(t, testcase.proto, serviceInterface.Protocol,
				"\n\nTest %s failure: Protocol was |%s| but should be |%s|.\n",
				testcase.namespace,
				serviceInterface.Protocol,
				testcase.proto)
			assert.Equal(t, testcase.port, serviceInterface.Port,
				"\n\nTest %s failure: Port was %d but should be %d.\n",
				testcase.namespace,
				serviceInterface.Port,
				testcase.port)
			assert.Assert(t, nil == serviceInterface.Headless,
				"\n\nTest %s failure: Headless was |%#v| but should be nil.\n",
				testcase.namespace,
				serviceInterface.Headless)
		}
	}
}
