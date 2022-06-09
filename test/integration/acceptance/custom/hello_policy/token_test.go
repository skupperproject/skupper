//go:build policy
// +build policy

package hello_policy

import (
	"path/filepath"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/token"
)

// Returns a cli.TestScenario for creating a token with/on the given:
// - name
// - path
// - cluster
// And check whether it works or is disallowed by policy
//
// Different from Hello World, the token created by this function can be
// reused 1000 times
func createTokenPolicyScenario(cluster *base.ClusterContext, prefix, testPath, name string, works bool) (createToken cli.TestScenario) {

	createToken = cli.TestScenario{
		Name: prefixName(prefix, "create-token"),
		Tasks: []cli.SkupperTask{
			{Ctx: cluster, Commands: []cli.SkupperCommandTester{
				// skupper token create - verify token has been created
				&token.CreateTester{
					Name:            name,
					FileName:        filepath.Join(testPath, name+".token.yaml"),
					PolicyProhibits: !works,
					// Here, we deviate from Hello World, as we're not testing expiry or uses.
					// This allows the token to be used repeatedly on some tests, saving
					// some time.
					Expiry: "600m",
					Uses:   "1000",
				},
			}},
		},
	}
	return
}
