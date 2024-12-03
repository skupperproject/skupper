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
		expectedOutput *HttpProxy
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
			expectedOutput: &HttpProxy{
				Host:        "bar",
				ServiceName: "baz",
				ServicePort: 1234,
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
				output := &HttpProxy{}
				output.readFromContourProxy(obj)
				assert.Equal(t, output.Host, tt.expectedOutput.Host)
				assert.Equal(t, output.ServiceName, tt.expectedOutput.ServiceName)
				assert.Equal(t, output.ServicePort, tt.expectedOutput.ServicePort)
			}
		})
	}
}

func Test_readFromContourProxy(t *testing.T) {
	testTable := []struct {
		name           string
		initialContent map[string]interface{}
		input          *HttpProxy
		expectedOutput *HttpProxy
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
			expectedOutput: &HttpProxy{
				Host:        "bar",
				ServiceName: "baz",
				ServicePort: 1234,
			},
		},
		{
			name: "bad spec field",
			initialContent: map[string]interface{}{
				"spec": false,
			},
			expectedError: ".spec.virtualhost accessor error",
		},
		{
			name: "bad tcpproxy field",
			initialContent: map[string]interface{}{
				"spec": map[string]interface{}{
					"tcpproxy": false,
				},
			},
			expectedError: ".spec.tcpproxy.services accessor error",
		},
		{
			name: "bad service name field",
			initialContent: map[string]interface{}{
				"spec": map[string]interface{}{
					"tcpproxy": map[string]interface{}{
						"services": listOfMaps(map[string]interface{}{
							"name": false,
						}),
					},
				},
			},
			expectedError: ".name accessor error",
		},
		{
			name: "bad service port field",
			initialContent: map[string]interface{}{
				"spec": map[string]interface{}{
					"tcpproxy": map[string]interface{}{
						"services": listOfMaps(map[string]interface{}{
							"port": false,
						}),
					},
				},
			},
			expectedError: ".port accessor error",
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetUnstructuredContent(tt.initialContent)
			if tt.input != nil {
				tt.input.writeToContourProxy(obj)
			}
			output := &HttpProxy{}
			err := output.readFromContourProxy(obj)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.Assert(t, err)
				assert.Equal(t, output.Host, tt.expectedOutput.Host)
				assert.Equal(t, output.ServiceName, tt.expectedOutput.ServiceName)
				assert.Equal(t, output.ServicePort, tt.expectedOutput.ServicePort)
			}
		})
	}
}

func listOfMaps(maps ...map[string]interface{}) []interface{} {
	var list []interface{}
	for _, m := range maps {
		list = append(list, m)
	}
	return list
}
