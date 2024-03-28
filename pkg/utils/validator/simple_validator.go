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
		return false, fmt.Errorf("value is not greater than zero")
	}
	return true, nil
}

///
