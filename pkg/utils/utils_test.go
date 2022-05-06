package utils

import (
	"gotest.tools/assert"
	"reflect"
	"testing"
)

func TestStringifySelector(t *testing.T) {
	type test struct {
		name   string
		labels map[string]string
		result string
	}

	testTable := []test{
		{name: "empty-map", labels: map[string]string{}, result: ""},
		{name: "one-label-map", labels: map[string]string{"label1": "value1"}, result: "label1=value1"},
		{name: "three-label-map", labels: map[string]string{"label1": "value1", "label2": "value2", "label3": "value3"}, result: "label1=value1,label2=value2,label3=value3"},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			// getting result into a map as labels ordering cannot be guaranteed
			expectedResMap := LabelToMap(test.result)
			actualResMap := LabelToMap(StringifySelector(test.labels))
			assert.Assert(t, reflect.DeepEqual(expectedResMap, actualResMap))
		})
	}
}

func TestSliceEquals(t *testing.T) {
	type test struct {
		name   string
		sliceA []string
		sliceB []string
		result bool
	}

	testTable := []test{
		{name: "not equals", sliceA: []string{"one", "two"}, sliceB: []string{"two", "three"}, result: false},
		{name: "not equals, one is empty", sliceA: []string{}, sliceB: []string{"two", "three"}, result: false},
		{name: "equals", sliceA: []string{"one", "two"}, sliceB: []string{"one", "two"}, result: true},
		{name: "equals but different order", sliceA: []string{"one", "two"}, sliceB: []string{"two", "one"}, result: true},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			expectedResult := test.result
			actualResult := StringSlicesEqual(test.sliceA, test.sliceB)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}
