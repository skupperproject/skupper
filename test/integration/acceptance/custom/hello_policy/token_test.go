//go:build policy
// +build policy

package hello_policy

import (
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/token"
)

// Returns a cli.TestScenario for creating a token with/on the given:
// - name
// - path
// - cluster
// And check whether it works or is disallowed by policy
func createTokenPolicyScenario(cluster *base.ClusterContext, prefix, testPath, name string, works bool) (createToken cli.TestScenario) {

	createToken = cli.TestScenario{
		Name: prefixName(prefix, "create-token"),
		Tasks: []cli.SkupperTask{
			{Ctx: cluster, Commands: []cli.SkupperCommandTester{
				// skupper token create - verify token has been created
				&token.CreateTester{
					Name:             name,
					FileName:         testPath + "/" + name + ".token.yaml",
					ExpectDisallowed: !works,
				},
			}},
		},
	}
	return
}
