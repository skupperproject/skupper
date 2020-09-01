package base

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

// TestFlags holds the common command line arguments
// needed for running tests. The goal is to keep it
// minimal. If tests allow custom configurations, it
// is better to define them through environment
// variables.
type testFlags struct {
	KubeConfigs     kubeConfigs
	EdgeKubeConfigs kubeConfigs
}

// Special type to allow multiple kubeconfig files to be provided
type kubeConfigs []string

// String returns string representing the provided contexts
func (k *kubeConfigs) String() string {
	var rep, sep string
	for _, name := range *k {
		rep += sep + name
		sep = ", "
	}
	return rep
}

// Set stores the provided kubeconfig entries into KubeConfigs
func (k *kubeConfigs) Set(file string) error {
	// validate if the provided file exists
	if _, err := os.Stat(file); err != nil {
		return err
	}
	// if file exists then use it
	*k = append(*k, file)
	return nil
}

var (
	TestFlags testFlags
)

func ParseFlags(m *testing.M) {
	// Registering flags to be parsed
	flag.Var(&TestFlags.KubeConfigs, "kubeconfig", "KUBECONFIG files to be used. You can provide the --kubeconfig flag multiple times.")
	flag.Var(&TestFlags.EdgeKubeConfigs, "edgekubeconfig", "Edge KUBECONFIG files to be used (other sites cannot connect to this cluster). You can provide the --edgekubeconfig flag multiple times.")

	// TODO evaluate if worth adding the following flag(s):
	// - KeepNamespaces bool

	// Parsing
	flag.Parse()

	// If only --edgekubeconfig provided, fail
	if len(TestFlags.KubeConfigs) == 0 && len(TestFlags.EdgeKubeConfigs) > 0 {
		panic("at least one --kubeconfig must be provided when using --edgekubeconfig")
	}
}

func setUnitTestFlags(public, private int) {
	TestFlags.KubeConfigs = []string{}
	TestFlags.EdgeKubeConfigs = []string{}
	for i := 0; i < public; i++ {
		TestFlags.KubeConfigs = append(TestFlags.KubeConfigs, fmt.Sprintf("kubeconfig.%d", i+1))
	}
	for i := 0; i < private; i++ {
		TestFlags.EdgeKubeConfigs = append(TestFlags.EdgeKubeConfigs, fmt.Sprintf("edgekubeconfig.%d", i+1))
	}
}
