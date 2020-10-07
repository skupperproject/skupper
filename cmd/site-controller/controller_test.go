package main

import (
	"flag"
	"os"
	"testing"
)

var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
