package validator

import (
	"gotest.tools/assert"
	"reflect"
	"regexp"
	"testing"
)

func TestNewStringValidator(t *testing.T) {

	t.Run("Test String Validator constructor", func(t *testing.T) {

		validRegexp, _ := regexp.Compile("^\\S*$")
		expectedResult := &StringValidator{validRegexp}
		actualResult := NewStringValidator()
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}

func TestStringValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "empty string", value: "", result: true},
		{name: "valid value", value: "provided-value", result: true},
		{name: "string with spaces", value: "provided value", result: false},
		{name: "string with numbers", value: "site123", result: true},
		{name: "number", value: 123, result: false},
		{name: "nil value", value: nil, result: false},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			stringValidator := NewStringValidator()
			expectedResult := test.result
			actualResult, _ := stringValidator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}

func TestNewNumberValidator(t *testing.T) {

	t.Run("Test Positive Int Validator constructor", func(t *testing.T) {

		expectedResult := &NumberValidator{PositiveInt: true}
		actualResult := NewNumberValidator()
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}

func TestIntegerValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "empty string", value: "", result: false},
		{name: "value greater than zero", value: 235, result: true},
		{name: "zero value", value: 0, result: true},
		{name: "negative number", value: -2, result: false},
		{name: "not valid characters", value: "abc", result: false},
		{name: "nil value", value: nil, result: false},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			numberValidator := NewNumberValidator()
			expectedResult := test.result
			actualResult, _ := numberValidator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}

func TestNewOptionValidator(t *testing.T) {

	t.Run("Test Option Validator constructor", func(t *testing.T) {

		expectedResult := &OptionValidator{
			AllowedOptions: []string{"a", "b"},
		}
		actualResult := NewOptionValidator([]string{"a", "b"})
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}

func TestOptionValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "empty string", value: "", result: false},
		{name: "value not included", value: "c", result: false},
		{name: "nil value", value: nil, result: false},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			optionValidator := NewOptionValidator([]string{"a", "b"})
			expectedResult := test.result
			actualResult, _ := optionValidator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}
