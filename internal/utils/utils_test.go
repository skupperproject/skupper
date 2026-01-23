package utils

import (
	"reflect"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
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

// LabelToMap expects label string to be a comma separated
// list of key and value pairs delimited by equals.
func LabelToMap(label string) map[string]string {
	m := map[string]string{}
	labels := strings.Split(label, ",")
	for _, l := range labels {
		if !strings.Contains(l, "=") {
			continue
		}
		entry := strings.Split(l, "=")
		m[entry[0]] = entry[1]
	}
	return m
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

func TestDefaultStr(t *testing.T) {
	testTable := []struct {
		name 	 string
		values []string
		result string
	}{
		{name: "four non-empty", values: []string{"foo", "bar", "hello", "world"}, result: "foo"},
		{name: "leading empty strings", values: []string{"", "", "test", "1"}, result: "test"},
		{name: "single non-empty string", values: []string{"single"}, result: "single"},
		{name: "empty at end, non-empty at start", values: []string{"first", "", "", ""}, result: "first"},
		{name: "spaces are not empty", values: []string{"", " ", "test"}, result: " "},
		{name: "two strings, first empty", values: []string{"", "second"}, result: "second"},
		{name: "two strings, both non-empty", values: []string{"first", "second"}, result: "first"},
		{name: "all empty strings", values: []string{"", "", ""}, result: ""},
		{name: "single empty string", values: []string{""}, result: ""},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t * testing.T) {
			expectedResult := test.result
			actualResult := DefaultStr(test.values...)
			assert.Equal(t, expectedResult, actualResult)
		})
	}
}