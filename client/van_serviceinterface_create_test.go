package client

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"

	"gotest.tools/assert"
)

var fp = fmt.Fprintf

func TestVanServiceInterfaceCreate(t *testing.T) {
	testcases := []struct {
		name  string
		doc   string
		init  bool
		err   string
		addr  string
		proto string
		port  int
	}{
		{
			name:  "vsic_1",
			doc:   "Uninitialized.",
			init:  false,
			addr:  "",
			proto: "",
			port:  0,
			err:   "Skupper not initialised in skupper",
		},
		{
			name:  "vsic_2",
			doc:   "Normal initialization.",
			init:  true,
			addr:  "half-addr",
			proto: "tcp",
			port:  666,
			err:   "",
		},
		{
			name:  "vsic_3",
			doc:   "Bad protocol.",
			init:  true,
			addr:  "half-addr",
			proto: "BISYNC",
			port:  1967,
			err:   "BISYNC is not a valid mapping. Choose 'tcp', 'http' or 'http2'.",
		},
		{
			name:  "vsic_4",
			doc:   "Bad port.",
			init:  true,
			addr:  "half-addr",
			proto: "tcp",
			port:  3141592653589,
			err:   "error",
		},
	}

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create the client
		cli, err := newMockClient("skupper", "", "")
		assert.Check(t, err, c.name)

		var service types.ServiceInterface

		if c.init {
			van := GetVanRouterSpecFromOpts(types.VanRouterCreateOptions{}, cli)
			dep, err := kube.NewTransportDeployment(van, cli.KubeClient)
			assert.Assert(t, cmp.Equal(err, nil))

			ownerRef := kube.GetOwnerReference(dep)

			kube.NewConfigMapWithOwner("skupper-services", ownerRef, van.Namespace, cli.KubeClient)

			service = types.ServiceInterface{
				Address:  c.addr,
				Protocol: c.proto,
				Port:     c.port,
			}
		} else {
			service = types.ServiceInterface{}
		}

		err = cli.VanServiceInterfaceCreate(ctx, &service)

		switch c.err {
		case "": // There should be no error.
			if err != nil {
				fp(os.Stdout, "Test %s failure: %s  An error was reported where none was expected.\n", c.name, c.doc)
			}
			assert.Assert(t, err == nil)
		case "error": // There should be some kind of error, but I don't know what it is.
			if err == nil {
				fp(os.Stdout, "Test %s failure: %s No error was reported, but one should have been.\n", c.name, c.doc)
			}
			assert.Assert(t, err != nil)
		default: // There should be this exact error.
			if c.err != err.Error() {
				fp(os.Stdout, "Test %s failure: %s The reported error was different from the expected error.\n", c.name, c.doc)
			}
			assert.Assert(t, c.err == err.Error())
		}
	}
}
