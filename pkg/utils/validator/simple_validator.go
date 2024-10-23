package validator

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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

func NewHostStringValidator() *StringValidator {
	re, err := regexp.Compile(`^[a-z0-9]+([-.]{1}[a-z0-9]+)*$`)
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

// TBD what are valid characters for selector field
func NewSelectorStringValidator() *StringValidator {
	re, err := regexp.Compile("^[A-Za-z0-9=:./-]+$")
	if err != nil {
		fmt.Printf("Error compiling regex: %v", err)
		return nil
	}
	return &StringValidator{
		Expression: re,
	}
}

func NewFilePathStringValidator() *StringValidator {
	re, err := regexp.Compile("^[A-Za-z0-9./~-]+$")
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
	IncludeZero bool
}

func NewNumberValidator() *NumberValidator {
	return &NumberValidator{
		PositiveInt: true,
		IncludeZero: true,
	}
}

func (i NumberValidator) Evaluate(value interface{}) (bool, error) {

	v, ok := value.(int)

	if !ok {
		return false, fmt.Errorf("value is not an integer")
	}

	if i.PositiveInt {
		if v < 0 {
			return false, fmt.Errorf("value is not positive")
		}
		if v > 0 {
			return true, nil
		}
		if v == 0 {
			if i.IncludeZero {
				return true, nil
			}
			return false, fmt.Errorf("value 0 is not allowed")
		}
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

type DurationValidator struct {
	MinDuration time.Duration
}

func NewTimeoutInSecondsValidator() *DurationValidator {
	return &DurationValidator{
		MinDuration: time.Second * 10,
	}
}

func (i DurationValidator) Evaluate(value time.Duration) (bool, error) {

	if value < i.MinDuration {
		return false, fmt.Errorf("duration must not be less than %v; got %v", i.MinDuration, value)
	}

	return true, nil
}

type WorkloadValidator struct {
	Expression     *regexp.Regexp
	AllowedOptions []string
}

func NewWorkloadStringValidator(validOptions []string) *WorkloadValidator {
	re, err := regexp.Compile("^[A-Za-z0-9.-_]+$")
	if err != nil {
		fmt.Printf("Error compiling regex: %v", err)
		return nil
	}
	return &WorkloadValidator{
		Expression:     re,
		AllowedOptions: validOptions,
	}
}

func (s WorkloadValidator) Evaluate(value interface{}) (string, string, bool, error) {

	v, ok := value.(string)

	if !ok {
		return "", "", false, fmt.Errorf("value is not a string")
	}

	// workload has two parts <resource-type>/<resource-name>
	resource := strings.Split(v, "/")
	if len(resource) != 2 {
		return "", "", false, fmt.Errorf("workload must include <resource-type>/<resource-name>")
	}

	if s.Expression.MatchString(resource[1]) {
		resourceType := strings.ToLower(resource[0])
		for _, option := range s.AllowedOptions {
			if option == resourceType {
				return option, resource[1], true, nil
			}
		}
	}
	return "", "", false, fmt.Errorf("value does not match this regular expression: %s", s.Expression)
}
