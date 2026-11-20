package securedaccess

import (
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_writeToContourProxy(t *testing.T) {
	testTable := []struct {
		name           string
		initialContent map[string]interface{}
		input          *HttpProxy
		expectedOutput map[string]interface{}
		expectedError  string
	}{
		{
			name:           "simple",
			initialContent: map[string]interface{}{},
			input: &HttpProxy{
				Host:        "bar",
				ServiceName: "baz",
				ServicePort: 1234,
			},
			expectedOutput: map[string]interface{}{
				"virtualhost": map[string]interface{}{"fqdn": "bar", "tls": map[string]interface{}{"passthrough": true}},
				"tcpproxy":    map[string]interface{}{"services": []interface{}{map[string]interface{}{"name": "baz", "port": int64(1234)}}},
			},
		},
		{
			name: "bad spec field",
			initialContent: map[string]interface{}{
				"spec": false,
			},
			input: &HttpProxy{
				Host:        "bar",
				ServiceName: "baz",
				ServicePort: 1234,
			},
			expectedError: "value cannot be set",
		},
		{
			name: "bad tls field",
			initialContent: map[string]interface{}{
				"spec": map[string]interface{}{
					"virtualhost": map[string]interface{}{
						"tls": false,
					},
				},
			},
			input: &HttpProxy{
				Host:        "bar",
				ServiceName: "baz",
				ServicePort: 1234,
			},
			expectedError: "value cannot be set",
		},
		{
			name: "bad tcpproxy field",
			initialContent: map[string]interface{}{
				"spec": map[string]interface{}{
					"tcpproxy": false,
				},
			},
			input: &HttpProxy{
				Host:        "bar",
				ServiceName: "baz",
				ServicePort: 1234,
			},
			expectedError: "value cannot be set",
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetUnstructuredContent(tt.initialContent)
			err := tt.input.writeToContourProxy(obj)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.Assert(t, err)
				assert.DeepEqual(t, obj.Object["spec"], tt.expectedOutput)
			}
		})
	}
}
