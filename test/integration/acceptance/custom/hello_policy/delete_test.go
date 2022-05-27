//go:build policy
// +build policy

package hello_policy

import (
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// Returns a test scenario that removes skupper from the given context
// (skupper delete) and then runs skupper status to confirm it's gone
func deleteSkupperTestScenario(ctx *base.ClusterContext, prefix string) (deleteSteps cli.TestScenario) {

	deleteSteps = cli.TestScenario{

		Name: prefixName(prefix, "skupper delete"),
		Tasks: []cli.SkupperTask{
			// skupper delete - delete and verify resources have been removed
			{
				Ctx: ctx, Commands: []cli.SkupperCommandTester{
					&cli.DeleteTester{},
					&cli.StatusTester{
						NotEnabled: true,
					},
				},
			},
		},
	}
	return
}
