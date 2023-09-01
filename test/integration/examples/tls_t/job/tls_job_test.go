//go:build job
// +build job

package job

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/test/integration/examples/tls_t"
)

func TestTlsJob(t *testing.T) {

	m := regexp.MustCompile("[ /]")
	for _, test := range tls_t.Tests {
		testName := fmt.Sprintf("server-%v-client-%v", test.Server.Options, strings.Join(test.Client.Options, "-"))
		testName = m.ReplaceAllLiteralString(testName, "-")

		t.Run(testName, func(t *testing.T) {
			addr := fmt.Sprintf("ssl-server:%v", test.Server.Port)
			result := tls_t.SendReceive(addr, test.Client.Options, test.Seek, testName)
			t.Logf("Success expected: %t; result: %v", test.Success, result)
			if (result == nil) != test.Success {
				t.Errorf("failed: client options: %v, server options: %v, result: %v", test.Client.Options, test.Server.Options, result)
			}
		})

	}

}
