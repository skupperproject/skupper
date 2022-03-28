package hello_policy

import (
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// Each policy piece has its own file.  On it, we define both the
// piece-specific tests _and_ the piece-specific infra.
//
// For example, the checking for link being (un)able to create or being
// destroyed is defined on functions on link_test.go
//
// These functions will take a cluster context and an optional name prefix.  It
// will return a slice of cli.TestScenario with the intended objective on the
// requested cluster, and the names of the individual scenarios will receive
// the prefix, if any given.  A use of that prefix would be, for example, to
// clarify that what's being checked is a 'side-effect' (eg when a link drops
// in a cluster because the policy was removed on the other cluster)

func testLinkPolicy(t *testing.T, pub, prv *base.ClusterContext) {

	initSteps := append(skupperInitInterior(pub), skupperInitEdge(prv)...)
	t.Run("init", func(t *testing.T) { cli.RunScenariosParallel(t, initSteps) })

	deleteSteps := append(deleteSkupper(pub), deleteSkupper(prv)...)
	t.Run("cleanup", func(t *testing.T) { cli.RunScenariosParallel(t, deleteSteps) })

}
