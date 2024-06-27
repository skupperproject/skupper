package validator

import (
	"fmt"
	"regexp"
)

type Validator interface {
	Evaluate(value interface{}) (bool, error)
}

//

type StringValidator struct {
	Expression *regexp.Regexp
}

func NewStringValidator() *StringValidator {
	re, err := regexp.Compile("^\\S*$")
	if err != nil {
		fmt.Printf("Error compiling regex: %v", err)
		return nil
	}
	return &StringValidator{
		Expression: re,
	}
}

func NewResourceStringValidator() *StringValidator {
	re, err := regexp.Compile("^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$")
	if err != nil {
		fmt.Printf("Error compiling regex: %v", err)
		return nil
	}
	return &StringValidator{
		Expression: re,
	}
}

func NewWorkloadStringValidator() *StringValidator {
	re, err := regexp.Compile("^[A-Za-z0-9=:./-]+$")
	if err != nil {
		fmt.Printf("Error compiling regex: %v", err)
		return nil
	}
	return &StringValidator{
		Expression: re,
	}
}

func (s StringValidator) Evaluate(value interface{}) (bool, error) {
	v, ok := value.(string)

	if !ok {
		return false, fmt.Errorf("value is not a string")
	}

	if s.Expression.MatchString(v) {
		return true, nil
	}

	return false, fmt.Errorf("value does not match this regular expression: %s", s.Expression)
}

//

type NumberValidator struct {
	PositiveInt bool
}

func NewNumberValidator() *NumberValidator {
	return &NumberValidator{
		PositiveInt: true,
	}
}

func (i NumberValidator) Evaluate(value interface{}) (bool, error) {

	v, ok := value.(int)

	if !ok {
		return false, fmt.Errorf("value is not an integer")
	}

	if i.PositiveInt {
		if v >= 0 {
			return true, nil
		}
		return false, fmt.Errorf("value is not positive")
	}
	return true, nil
}

///

type OptionValidator struct {
	AllowedOptions []string
}

func NewOptionValidator(validOptions []string) *OptionValidator {
	return &OptionValidator{
		AllowedOptions: validOptions,
	}
}

func (i OptionValidator) Evaluate(value interface{}) (bool, error) {

	v, ok := value.(string)

	if !ok {
		return false, fmt.Errorf("value is not a string")
	}

	if v == "" {
		return false, fmt.Errorf("value must not be empty")
	}

	valueFound := false
	for _, option := range i.AllowedOptions {
		if option == v {
			valueFound = true
		}
	}

	if !valueFound {
		return false, fmt.Errorf("value %s not allowed. It should be one of this options: %v", v, i.AllowedOptions)
	}
	return true, nil
}
