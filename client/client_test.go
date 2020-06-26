package client

import (
	"flag"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
)

var trans = cmp.Transformer("Sort", func(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
})

func TestNewClient(t *testing.T) {
	testcases := []struct {
		doc             string
		namespace       string
		context         string
		kubeConfigPath  string
		expectedError   string
		expectedVersion string
	}{
		{
			namespace:      "skupper",
			context:        "",
			kubeConfigPath: "",
			expectedError:  "",
			doc:            "test one",
		},
	}

	for _, c := range testcases {
		_, err := newMockClient(c.namespace, c.context, c.kubeConfigPath)
		assert.Check(t, err, c.doc)
	}
}

var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
