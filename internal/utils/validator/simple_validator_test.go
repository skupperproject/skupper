package validator

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"gotest.tools/v3/assert"
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

		validRegexp := regexp.MustCompile("^[A-Za-z0-9._-]+$")
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
		{name: "string with dashes", value: "a/workload-with-dashes", result: true},
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

func TestNewInputResourceFilenameValidator(t *testing.T) {

	t.Run("Test New Input Resource Filename Validator constructor", func(t *testing.T) {

		validRegexp := regexp.MustCompile(`^([A-Z][a-zA-Z]*)-(.+)\.(yaml|yml|json)$`)
		expectedResourceTypes := []string{
			"Site",
			"Listener",
			"Connector",
			"RouterAccess",
			"AccessGrant",
			"Link",
			"AccessToken",
			"Certificate",
			"SecuredAccess",
			"MultiKeyListener",
			"Secret",
		}
		expectedResult := &InputResourceFilenameValidator{validRegexp, expectedResourceTypes}
		actualResult := NewInputResourceFilenameValidator()
		assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
	})
}

func TestInputResourceFilenameValidator_Evaluate(t *testing.T) {
	type test struct {
		name   string
		value  interface{}
		result bool
	}

	testTable := []test{
		{name: "valid Site yaml", value: "Site-mysite.yaml", result: true},
		{name: "valid Connector yml", value: "Connector-backend.yml", result: true},
		{name: "valid Listener json", value: "Listener-frontend.json", result: true},
		{name: "valid RouterAccess", value: "RouterAccess-myaccess.yaml", result: true},
		{name: "valid AccessGrant", value: "AccessGrant-grant1.yaml", result: true},
		{name: "valid Link", value: "Link-mylink.yaml", result: true},
		{name: "valid AccessToken", value: "AccessToken-token1.yaml", result: true},
		{name: "valid Certificate", value: "Certificate-cert1.yaml", result: true},
		{name: "valid SecuredAccess", value: "SecuredAccess-secure1.yaml", result: true},
		{name: "valid MultiKeyListener", value: "MultiKeyListener-listener1.yaml", result: true},
		{name: "valid Secret", value: "Secret-mysecret.yaml", result: true},
		{name: "valid with dashes in name", value: "Site-my-site-name.yaml", result: true},
		{name: "valid with numbers in name", value: "Connector-backend123.yaml", result: true},
		{name: "invalid resource type", value: "Invalid-name.yaml", result: false},
		{name: "lowercase resource type", value: "site-name.yaml", result: false},
		{name: "missing resource type", value: "name.yaml", result: false},
		{name: "missing name", value: "Site-.yaml", result: false},
		{name: "no extension", value: "Site-name", result: false},
		{name: "wrong extension", value: "Site-name.txt", result: false},
		{name: "empty string", value: "", result: false},
		{name: "number", value: 123, result: false},
		{name: "nil value", value: nil, result: false},
		{name: "no hyphen separator", value: "Sitename.yaml", result: false},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			validator := NewInputResourceFilenameValidator()
			expectedResult := test.result
			actualResult, _ := validator.Evaluate(test.value)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}
