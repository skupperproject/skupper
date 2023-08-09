package formatter

import (
	"fmt"
	"gotest.tools/assert"
	"testing"
)

func TestServicePrinter(t *testing.T) {
	tests := []struct {
		name          string
		list          *list
		expectedJson  string
		expectedYaml  string
		expectedError error
	}{
		{
			name:          "test 1 - empty list",
			list:          &list{},
			expectedError: fmt.Errorf("error before marshalling: empty list"),
		},
		{
			name:         "test 2 - one service",
			list:         getTest2List(),
			expectedJson: getTest2JsonResult(),
			expectedYaml: getTest2YamlResult(),
		},
		{
			name:         "test 3 - one service with labels",
			list:         getTest3List(),
			expectedJson: getTest3JsonResult(),
			expectedYaml: getTest3YamlResult(),
		},
		{
			name:         "test 4 - many services",
			list:         getTest4List(),
			expectedJson: getTest4JsonResult(),
			expectedYaml: getTest4YamlResult(),
		},
	}
	for _, tc := range tests {

		t.Run(fmt.Sprintf(tc.name), func(t *testing.T) {
			printer := ServiceStatusPrinter{
				OriginalData: tc.list,
			}

			result, err := printer.PrintJsonFormat()

			if tc.expectedError != nil {
				assert.Equal(t, err.Error(), tc.expectedError.Error())
			} else {
				assert.Assert(t, err)
				assert.Equal(t, result, tc.expectedJson)
			}

			result, err = printer.PrintYamlFormat()
			if tc.expectedError != nil {
				assert.Equal(t, err.Error(), tc.expectedError.Error())
			} else {
				assert.Assert(t, err)
				assert.Equal(t, result, tc.expectedYaml)
			}

		})

	}

}

func getTest2List() *list {

	l := NewList()
	l.Item("Services exposed through Skupper:")
	svc := l.NewChild("service (http2 port 5001)")
	targets := svc.NewChild("Targets:")
	targets.NewChild("app=serviceapp name=service namespace=namespace1")

	return l
}

func getTest3List() *list {

	l := NewList()
	l.Item("Services exposed through Skupper:")
	svc := l.NewChild("service (http2 port 5001)")
	targets := svc.NewChild("Targets:")
	targets.NewChild("app=serviceapp name=service namespace=namespace1")
	labels := svc.NewChild("Labels:")
	labels.NewChild("label1=value1")
	labels.NewChild("label2=value2")

	return l
}

func getTest4List() *list {

	l := NewList()
	l.Item("Services exposed through Skupper:")
	svc := l.NewChild("service (http2 port 5001)")
	targets := svc.NewChild("Targets:")
	targets.NewChild("app=serviceapp name=service namespace=namespace1")
	labels := svc.NewChild("Labels:")
	labels.NewChild("label1=value1")
	labels.NewChild("label2=value2")

	svc2 := l.NewChild("service2 (tcp port 8080 8081)")
	targets2 := svc2.NewChild("Targets:")
	targets2.NewChild("name=service2 namespace=namespace2")

	l.NewChild("service3 (http port 4040)")

	return l
}

func getTest2JsonResult() string {
	return `[
  {
    "address": "service",
    "protocol": "http2",
    "port": [
      "5001"
    ],
    "targets": [
      {
        "app": "serviceapp",
        "name": "service",
        "namespace": "namespace1"
      }
    ]
  }
]`
}

func getTest3JsonResult() string {
	return `[
  {
    "address": "service",
    "protocol": "http2",
    "port": [
      "5001"
    ],
    "targets": [
      {
        "app": "serviceapp",
        "name": "service",
        "namespace": "namespace1"
      }
    ],
    "labels": [
      {
        "label1": "value1"
      },
      {
        "label2": "value2"
      }
    ]
  }
]`
}

func getTest4JsonResult() string {
	return `[
  {
    "address": "service",
    "protocol": "http2",
    "port": [
      "5001"
    ],
    "targets": [
      {
        "app": "serviceapp",
        "name": "service",
        "namespace": "namespace1"
      }
    ],
    "labels": [
      {
        "label1": "value1"
      },
      {
        "label2": "value2"
      }
    ]
  },
  {
    "address": "service2",
    "protocol": "tcp",
    "port": [
      "8080",
      "8081"
    ],
    "targets": [
      {
        "name": "service2",
        "namespace": "namespace2"
      }
    ]
  },
  {
    "address": "service3",
    "protocol": "http",
    "port": [
      "4040"
    ]
  }
]`
}

func getTest2YamlResult() string {
	return `- address: service
  protocol: http2
  port:
  - "5001"
  targets:
  - app: serviceapp
    name: service
    namespace: namespace1
`
}

func getTest3YamlResult() string {
	return `- address: service
  protocol: http2
  port:
  - "5001"
  targets:
  - app: serviceapp
    name: service
    namespace: namespace1
  labels:
  - label1: value1
  - label2: value2
`
}

func getTest4YamlResult() string {
	return `- address: service
  protocol: http2
  port:
  - "5001"
  targets:
  - app: serviceapp
    name: service
    namespace: namespace1
  labels:
  - label1: value1
  - label2: value2
- address: service2
  protocol: tcp
  port:
  - "8080"
  - "8081"
  targets:
  - name: service2
    namespace: namespace2
- address: service3
  protocol: http
  port:
  - "4040"
`
}
