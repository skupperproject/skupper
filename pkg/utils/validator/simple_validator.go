package validator

import (
	"fmt"
	"regexp"
	"slices"
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

func (s StringValidator) Evaluate(value interface{}) (bool, error) {
	v, ok := value.(string)

	if !ok {
		return false, fmt.Errorf("value is not a string")
	}

	if s.Expression.MatchString(v) {
		return true, nil
	}

	return false, fmt.Errorf("value contains spaces")
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

	if !slices.Contains(i.AllowedOptions, v) {
		return false, fmt.Errorf("value %s not allowed. It should be one of this options: %v", v, i.AllowedOptions)
	}
	return true, nil
}
