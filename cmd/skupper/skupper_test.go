package main

import (
	"flag"
	"os"
	"testing"

	"gotest.tools/assert"
)

func Test_parseTargetTypeAndName(t *testing.T) {
	targetType, targetName := parseTargetTypeAndName([]string{"type", "name"})
	assert.Equal(t, targetType, "type")
	assert.Equal(t, targetName, "name")

	targetType, targetName = parseTargetTypeAndName([]string{"type/name"})
	assert.Equal(t, targetType, "type")
	assert.Equal(t, targetName, "name")
}

func Test_bindArgs(t *testing.T) {
	genericError := "Service name, target type and target name must all be specified (e.g. 'skupper bind <service-name> <target-type> <target-name>')"
	b := func(args []string) error {
		return bindArgs(nil, args)
	}

	assert.Error(t, b([]string{}), genericError)
	assert.Error(t, b([]string{"oneArg"}), genericError)
	assert.Error(t, b([]string{"one/Arg"}), genericError)
	assert.Error(t, b([]string{"one", "resource"}), genericError)

	//must this fail?
	//assert.Error(t, b([]string{"one/two", "resource/name"}), genericError)

	assert.Assert(t, b([]string{"one", "resource/name"}))
	//note  illegal vs extra
	assert.Error(t, b([]string{"one", "resource/name", "three"}), "extra argument: three")
	assert.Error(t, b([]string{"one", "resource/name", "three", "four"}), "illegal argument: four")
	assert.Error(t, b([]string{"one", "resource/name", "three", "four", "five"}), "illegal argument: four")

	assert.Assert(t, b([]string{"one", "resource", "name"}))
	assert.Error(t, b([]string{"one", "resource", "name", "four"}), "illegal argument: four")
	assert.Error(t, b([]string{"one", "resource", "name", "four", "five"}), "illegal argument: four")
}

func Test_createServiceArgs(t *testing.T) {
	c := func(args []string) error {
		return createServiceArgs(nil, args)
	}

	assert.Error(t, c([]string{}), "Name and port must be specified")
	assert.Error(t, c([]string{"noport"}), "Name and port must be specified")

	assert.Assert(t, c([]string{"service:port"}))

	assert.Error(t, c([]string{"service:port", "other"}), "extra argument: other")
	assert.Error(t, c([]string{"service:port", "other", "arg"}), "illegal argument: arg")

	assert.Assert(t, c([]string{"service", "port"}))
	assert.Error(t, c([]string{"service", "port", "other"}), "illegal argument: other")
	assert.Error(t, c([]string{"service", "port", "other", "arg"}), "illegal argument: other")
}

func Test_exposeTargetArgs(t *testing.T) {
	genericError := "expose target and name must be specified (e.g. 'skupper expose deployment <name>'"
	targetError := "expose target type must be one of: [deployment, statefulset, pods, service]"

	e := func(args []string) error {
		return exposeTargetArgs(nil, args)
	}

	assert.Error(t, e([]string{}), genericError)
	assert.Error(t, e([]string{"depl/name"}), targetError)

	assert.Error(t, e([]string{"depl/name", "two"}), "extra argument: two")
	assert.Error(t, e([]string{"depl/name", "two", "three"}), "illegal argument: three")
	assert.Error(t, e([]string{"depl/name", "two", "three", "four"}), "illegal argument: three")

	assert.Error(t, e([]string{"depl/name"}), targetError)
	assert.Error(t, e([]string{"anything", "name"}), targetError)

	assert.Error(t, e([]string{"deployment"}), genericError)

	assert.Assert(t, e([]string{"deployment", "name"}))

	assert.Error(t, e([]string{"deployment", "name", "three"}), "illegal argument: three")
	assert.Error(t, e([]string{"deployment", "name", "three", "four"}), "illegal argument: three")

	for _, target := range validExposeTargets {
		assert.Assert(t, e([]string{target, "name"}))
	}
}

var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
