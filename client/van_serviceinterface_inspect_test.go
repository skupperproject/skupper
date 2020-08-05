package client

import (
	"context"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"

	"gotest.tools/assert"
)

func TestVanServiceInterfaceInspect(t *testing.T) {
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
			err = cli.VanRouterCreate(ctx, types.VanSiteConfig{
				Spec: types.VanSiteConfigSpec{
					SkupperName:       testcase.namespace,
					IsEdge:            false,
					EnableController:  true,
					EnableServiceSync: true,
					EnableConsole:     false,
					AuthMode:          "",
					User:              "",
					Password:          "",
					ClusterLocal:      true,
				},
			})
			assert.Check(t, err, "%s: Unable to create VAN router", testcase.namespace)
		}

		// Create the VanServiceInterface.
		service := types.ServiceInterface{
			Address:  testcase.addr,
			Protocol: testcase.proto,
			Port:     testcase.port,
		}
		err = cli.VanServiceInterfaceCreate(ctx, &service)

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
			assert.Check(t, err, "Creation failed.")

			// When we inspect the VanServiceInterface, make sure that the
			// expected values have been set.
			serviceInterface, err := cli.VanServiceInterfaceInspect(ctx, testcase.addr)
			assert.Check(t, err, "Inspectionion failed.")

			assert.Equal(t, testcase.addr, serviceInterface.Address, "Error in address.")
			assert.Equal(t, testcase.proto, serviceInterface.Protocol, "Error in protocol.")
			assert.Assert(t, nil == serviceInterface.Headless, "Error in headless.")
		}
	}
}
