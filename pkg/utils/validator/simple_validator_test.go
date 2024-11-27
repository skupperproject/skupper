package validator

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestNewStringValidator(t *testing.T) {

	t.Run("Test String Validator constructor", func(t *testing.T) {

		validRegexp := regexp.MustCompile(`^\S*$`)
		expectedResult := &stringValidator{validRegexp}
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

		expectedResult := &NumberValidator{PositiveInt: true, IncludeZero: true}
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

func TestTimeoutInSecondsValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  time.Duration
		result bool
	}

	testTable := []test{
		{name: "value less than minimum", value: time.Second * 1, result: false},
		{name: "zero value", value: time.Second * 0, result: false},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			numberValidator := NewTimeoutInSecondsValidator()

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

func TestNewResourceStringValidator(t *testing.T) {

	t.Run("Test New Resource String Validator constructor", func(t *testing.T) {

		validRegexp := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$`)
		expectedResult := &stringValidator{validRegexp}
		actualResult := NewResourceStringValidator()
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}

func TestNewResourceStringValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "empty string", value: "", result: false},
		{name: "valid value", value: "provided-value", result: true},
		{name: "string with spaces", value: "provided value", result: false},
		{name: "string with numbers", value: "site123", result: true},
		{name: "number", value: 123, result: false},
		{name: "nil value", value: nil, result: false},
		{name: "string with underscore", value: "abc_def", result: false},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			stringValidator := NewResourceStringValidator()
			expectedResult := test.result
			actualResult, _ := stringValidator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}

func TestNewSelectorStringValidator(t *testing.T) {

	t.Run("Test New Selector String Validator constructor", func(t *testing.T) {

		validRegexp := regexp.MustCompile("^[A-Za-z0-9=:./-]+$")
		expectedResult := &stringValidator{validRegexp}
		actualResult := NewSelectorStringValidator()
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}

func TestNewSelectorStringValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "empty string", value: "", result: false},
		{name: "string with spaces", value: "provided value", result: false},
		{name: "string with numbers", value: "site123", result: true},
		{name: "number", value: 123, result: false},
		{name: "nil value", value: nil, result: false},
		{name: "string with underscore", value: "abc_def", result: false},
		{name: "string with equal", value: "abc=def", result: true},
		{name: "string with slash", value: "abc/def", result: true},
		{name: "string with dash", value: "abc-def", result: true},
		{name: "string with dot", value: "provided.value", result: true},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			stringValidator := NewSelectorStringValidator()
			expectedResult := test.result
			actualResult, _ := stringValidator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}

func TestNewFilePathStringValidator(t *testing.T) {

	t.Run("Test New File Path String Validator constructor", func(t *testing.T) {

		validRegexp := regexp.MustCompile("^[A-Za-z0-9./~-]+$")
		expectedResult := &stringValidator{validRegexp}
		actualResult := NewFilePathStringValidator()
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}

func TestNewFilePathStringValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "empty string", value: "", result: false},
		{name: "string with spaces", value: "provided value", result: false},
		{name: "string with numbers", value: "site123", result: true},
		{name: "number", value: 123, result: false},
		{name: "nil value", value: nil, result: false},
		{name: "string with underscore", value: "abc_def", result: false},
		{name: "string with equal", value: "abc=def", result: false},
		{name: "string with slash", value: "abc/def", result: true},
		{name: "string with dash", value: "abc-def", result: true},
		{name: "string with dot", value: "provided.value", result: true},
		{name: "valid path", value: "~/tmp/test.yaml", result: true},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			stringValidator := NewFilePathStringValidator()
			expectedResult := test.result
			actualResult, _ := stringValidator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}

func TestNewWorkloadStringValidator(t *testing.T) {

	t.Run("Test New Workload String Validator constructor", func(t *testing.T) {

		validRegexp := regexp.MustCompile("^[A-Za-z0-9.-_]+$")
		expectedResult := &WorkloadValidator{validRegexp, []string{"a", "b"}}
		actualResult := NewWorkloadStringValidator([]string{"a", "b"})
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}
func TestWorkloadStringValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "empty string", value: "", result: false},
		{name: "valid value", value: "a/name", result: true},
		{name: "string without /", value: "aname", result: false},
		{name: "string with numbers", value: "b/name123", result: true},
		{name: "number", value: 123, result: false},
		{name: "nil value", value: nil, result: false},
		{name: "string without name", value: "a/", result: false},
		{name: "string without type", value: "/name", result: false},
		{name: "string non matching type", value: "c/name", result: false},
		{name: "bad value", value: "a/name@#", result: false},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			stringValidator := NewWorkloadStringValidator([]string{"a", "b"})
			expectedResult := test.result
			_, _, actualResult, _ := stringValidator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}
