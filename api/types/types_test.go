package types

import (
	jsonencoding "encoding/json"
	"fmt"
	"testing"

	"gotest.tools/assert"
)

var jsonDefs = fmt.Sprintf(`[%s, %s]`, jsonDef, jsonDef)
var jsonDef = `{
    "address": "my-app",
    "protocol": "http",
    "ports": [8080, 9090],
	"eventChannel": true,
	"aggregate": "json",
	"headless": {
		"name": "headless-1",
		"size": 2,
		"targetPorts": {
			"8080": 8181,
			"9090": 9191
		},
		"affinity": {
			"aff1": "valaff1"
		},
		"antiAffinity": {
			"aaff1": "valaaff1"
		},
		"nodeSelector": {
			"node": "node1"
		},
		"cpuRequest": "10",
		"memoryRequest": "100"
	},
    "labels": {
        "app": "my-app"
    },
    "targets": [
        {
            "name": "my-app",
            "selector": "app=my-app",
			"targetPorts": {
				"8080": 8181,
				"9090": 9191
			},
			"service": "service"
        }
    ],
    "origin": "site1"
}`

var jsonDefsV1 = fmt.Sprintf(`[%s, %s]`, jsonDefV1, jsonDefV1)
var jsonDefV1 = `{
    "address": "my-app",
    "protocol": "http",
    "port": 9090,
	"eventChannel": true,
	"aggregate": "json",
	"headless": {
		"name": "headless-1",
		"size": 2,
		"targetPort": 8181,
		"affinity": {
			"aff1": "valaff1"
		},
		"antiAffinity": {
			"aaff1": "valaaff1"
		},
		"nodeSelector": {
			"node": "node1"
		},
		"cpuRequest": "10",
		"memoryRequest": "100"
	},
    "labels": {
        "app": "my-app"
    },
    "targets": [
        {
            "name": "my-app",
            "selector": "app=my-app",
            "targetPort": 8080,
			"service": "service"
        }
    ],
    "origin": "site1"
}`
var jsonDefV1Converted = `{
    "address": "my-app",
    "protocol": "http",
    "ports": [9090],
	"eventChannel": true,
	"aggregate": "json",
	"headless": {
		"name": "headless-1",
		"size": 2,
		"targetPorts": {
			"9090": 8181
		},
		"affinity": {
			"aff1": "valaff1"
		},
		"antiAffinity": {
			"aaff1": "valaaff1"
		},
		"nodeSelector": {
			"node": "node1"
		},
		"cpuRequest": "10",
		"memoryRequest": "100"
	},
    "labels": {
        "app": "my-app"
    },
    "targets": [
        {
            "name": "my-app",
            "selector": "app=my-app",
            "targetPorts": {
				"9090": 8080
			},
			"service": "service"
        }
    ],
    "origin": "site1"
}`

func TestServiceInterfaceList_ConvertFrom(t *testing.T) {
	// Using the latest model for comparison
	svcModel := ServiceInterface{}
	assert.Assert(t, jsonencoding.Unmarshal([]byte(jsonDef), &svcModel))
	fmt.Println(svcModel.Headless)

	// Model to compare with V1
	svcModelV1 := ServiceInterface{}
	assert.Assert(t, jsonencoding.Unmarshal([]byte(jsonDefV1Converted), &svcModelV1))
	fmt.Println(svcModel.Headless)

	type test struct {
		doc              string
		serviceDefs      string
		expectedModel    ServiceInterface
		expectedServices int
	}

	tests := []test{
		{
			doc:              "current-version",
			serviceDefs:      jsonDefs,
			expectedModel:    svcModel,
			expectedServices: 2,
		},
		{
			doc:              "v1",
			serviceDefs:      jsonDefsV1,
			expectedModel:    svcModelV1,
			expectedServices: 2,
		},
	}
	for _, ti := range tests {
		t.Run(ti.doc, func(t *testing.T) {
			svcList := &ServiceInterfaceList{}
			assert.Assert(t, svcList.ConvertFrom(ti.serviceDefs))
			assert.Equal(t, len(*svcList), ti.expectedServices)
			assert.DeepEqual(t, *svcList, ServiceInterfaceList{ti.expectedModel, ti.expectedModel})
			for _, svc := range *svcList {
				assert.DeepEqual(t, svc, ti.expectedModel)
				assert.DeepEqual(t, svc.Headless, ti.expectedModel.Headless)
				assert.DeepEqual(t, svc.Targets, ti.expectedModel.Targets)
			}
		})
	}
}
