// +build integration

package edgecon

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
)

var verbose bool = true
var red string = "\033[1;31m"
var green string = "\033[1;32m"
var cyan string = "\033[1;36m"
var yellow string = "\033[1;33m"
var resetColor string = "\033[0m"

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestEdgeConnectivity(t *testing.T) {
	// In this test there is always one private namespace,
	// and it is always an edge.
	there_can_be_only_1 := int32(1)

	testcases := []TestCase{
		// Test 1 -------------------------------------------------------
		{
			name:    "one-direct",
			diagram: []string{"edge  -->  interior"},
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            true,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          there_can_be_only_1,
			},
			public_public_cnx: map[int]int{},
			// The IDs on clusters are 1-based, not 0-based.
			private_public_cnx: []int{1},
			direct_count:       1,
			indirect_count:     0,
		},

		// Test 2 -------------------------------------------------------
		{
			name: "two-direct-V",
			diagram: []string{"edge  -->  interior-1",
				"edge  -->  interior-2"},
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          2,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            true,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          there_can_be_only_1,
			},
			public_public_cnx: map[int]int{},
			// The IDs on clusters are 1-based, not 0-based.
			private_public_cnx: []int{1, 2},
			direct_count:       2,
			indirect_count:     0,
		},

		// Test 3 -------------------------------------------------------
		{
			name: "two-direct-triangle",
			diagram: []string{"edge  -->  interior-1",
				"edge  -->  interior-2",
				"interior-1  --> interior-2"},
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          2,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            true,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          there_can_be_only_1,
			},
			public_public_cnx: map[int]int{1: 2},
			// The IDs on clusters are 1-based, not 0-based.
			private_public_cnx: []int{1, 2},
			direct_count:       2,
			indirect_count:     0,
		},

		// Test 4 -------------------------------------------------------
		{
			name: "three-direct",
			diagram: []string{"interior-1  -->  interior-2",
				"interior-2  -->  interior-3",
				"edge  -->  interior-1",
				"edge  -->  interior-2",
				"edge  -->  interior-3"},
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          3,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            true,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          there_can_be_only_1,
			},
			public_public_cnx: map[int]int{1: 2, 2: 3},
			// The IDs on clusters are 1-based, not 0-based.
			private_public_cnx: []int{1, 2, 3},
			direct_count:       3,
			indirect_count:     0,
		},

		// Test 5 -------------------------------------------------------
		{
			name: "one-direct-one-indirect",
			diagram: []string{"edge  -->  interior-1",
				"interior-1  -->  interior-2"},
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          2,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            true,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "",
				Password:          "",
				Ingress:           types.IngressNoneString,
				Replicas:          there_can_be_only_1,
			},
			public_public_cnx: map[int]int{1: 2},
			// The IDs on clusters are 1-based, not 0-based.
			private_public_cnx: []int{1},
			direct_count:       1,
			indirect_count:     1,
		},
	}

	for test_index, testcase := range testcases {
		t.Logf("Testing: %s\n", testcase.name)
		if verbose {
			fp(os.Stdout, "\n\n%stest %d: %s%s%s\n", yellow, test_index+1, cyan, testcase.name, resetColor)
			fp(os.Stdout, "%s", cyan)
			for _, s := range testcase.diagram {
				fp(os.Stdout, "\t%s\n", s)
			}
			fp(os.Stdout, "\n\tdirect: %d   indirect: %d\n", testcase.direct_count, testcase.indirect_count)
			fp(os.Stdout, "%s\n\n", resetColor)
		}

		needs := base.ClusterNeeds{
			NamespaceId:     "edgecon",
			PublicClusters:  int(testcase.createOptsPublic.Replicas),
			PrivateClusters: int(testcase.createOptsPrivate.Replicas),
		}
		testRunner := &EdgeConnectivityTestRunner{}
		testRunner.BuildOrSkip(t, needs, nil)
		ctx, cancel := context.WithCancel(context.Background())
		base.HandleInterruptSignal(t, func(t *testing.T) {
			testRunner.TearDown(ctx, &testcase)
			cancel()
		})
		testRunner.Run(ctx, &testcase, t)
	}
}
