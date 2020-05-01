package client

import (
	"testing"

	"gotest.tools/assert"
)

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
