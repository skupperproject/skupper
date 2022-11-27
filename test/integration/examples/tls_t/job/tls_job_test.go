//+build job

package job

import (
	"fmt"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tls_t"
)

func TestTlsJob(t *testing.T) {

	for _, test := range tls_t.Tests {
		testName := fmt.Sprintf("server-%v-client-%v", test.Server.Options, strings.Join(test.Client.Options, "-"))
		testName = strings.ReplaceAll(testName, " ", "-")

		t.Run(testName, func(t *testing.T) {
			// TODO: move string to package var?
			addr := fmt.Sprintf("ssl-server:%v", test.Server.Port)
			result := tls_t.SendReceive(addr, test.Client.Options)
			t.Logf("Success expected: %t; result: %v", test.Success, result)
			if (result == nil) != test.Success {
				t.Errorf("failed: client options: %v, server options: %v, result: %v", test.Client.Options, test.Server.Options, result)
			}
		})

	}

}
