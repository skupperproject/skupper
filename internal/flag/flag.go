package flag

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func StringVar(flags *flag.FlagSet, output *string, flagName string, envVarName string, defaultValue string, usage string) {
	flags.StringVar(output, flagName, stringEnvVar(envVarName, defaultValue), usage)
}

func BoolVar(flags *flag.FlagSet, output *bool, flagName string, envVarName string, defaultValue bool, usage string) error {
	dval, err := boolEnvVar(envVarName, defaultValue)
	//set flag inspite of error, caller can decide whether to ignore and go with default or not
	flags.BoolVar(output, flagName, dval, usage)
	return err
}

func IntVar(flags *flag.FlagSet, output *int, flagName string, envVarName string, defaultValue int, usage string) error {
	dval, err := intEnvVar(envVarName, defaultValue)
	//set flag inspite of error, caller can decide whether to ignore and go with default or not
	flags.IntVar(output, flagName, dval, usage)
	return err
}

func MultiStringVar(flags *flag.FlagSet, output *[]string, flagName string, envVarName string, defaultValue []string, usage string) {
	ms := &multistring{
		output: output,
	}
	*(ms.output) = multiStringEnvVar(envVarName, defaultValue)
	flags.Var(ms, flagName, usage)
}

type multistring struct {
	output *[]string
}

func (i *multistring) String() string {
	if i.output == nil {
		return ""
	}
	return strings.Join(*(i.output), ",")
}

func (i *multistring) Set(value string) error {
	if i.output != nil {
		*(i.output) = strings.Split(value, ",")
	}
	return nil
}

func intEnvVar(name string, defaultValue int) (int, error) {
	if svalue, ok := os.LookupEnv(name); ok {
		value, err := strconv.Atoi(svalue)
		if err != nil {
			return defaultValue, fmt.Errorf("Bad value for %q: %s", name, err)
		}
		return value, nil
	}
	return defaultValue, nil
}

func boolEnvVar(name string, defaultValue bool) (bool, error) {
	if svalue, ok := os.LookupEnv(name); ok {
		value, err := strconv.ParseBool(svalue)
		if err != nil {
			return defaultValue, fmt.Errorf("Bad value for %q: %s", name, err)
		}
		return value, nil
	}
	return defaultValue, nil
}

func stringEnvVar(name string, defaultValue string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaultValue
}

func multiStringEnvVar(name string, defaultValue []string) []string {
	if value, ok := os.LookupEnv(name); ok {
		return strings.Split(value, ",")
	}
	return defaultValue
}
