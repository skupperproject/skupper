package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
)

/*================================================================
  Multiple tests of different topologies, in each case with
  a single edge.
  Build the topology, then ask the edge about its connectivity.
  See if it has the right number of direct and indirect
  connections.
================================================================*/

var fp = fmt.Fprintf

var verbose bool = true
var red string = "\033[1;31m"
var green string = "\033[1;32m"
var cyan string = "\033[1;36m"
var resetColor string = "\033[0m"

type SkupperNamespace struct {
	name   string
	isEdge bool

	client         *VanClient
	secretFileName string
}

type Connexion struct {
	from string
	to   string
}

type TopologyTest struct {
	name             string
	diagram          []string
	namespaces       []*SkupperNamespace
	connexions       []Connexion
	direct, indirect int // These are the expected results.
}

func getNamespace(targetNamespace string, t *TopologyTest) *SkupperNamespace {
	for _, ns := range t.namespaces {
		if ns.name == targetNamespace {
			return ns
		}
	}
	fp(os.Stdout, "getNamespace error: can't find client |%s|\n", targetNamespace)
	os.Exit(1)
	return nil
}

func getEdge(t *TopologyTest) *SkupperNamespace {
	for _, ns := range t.namespaces {
		if strings.Contains(ns.name, "edge") {
			return ns
		}
	}
	return nil
}

func getConnectivity(
	testName string,
	namespace *SkupperNamespace,
	ctx context.Context,
	attempts int,
	expectedDirect, expectedIndirect int) (direct, indirect int, err error) {

	for {
		if attempts <= 0 {
			break
		}
		info(testName, "Checking connectivity...")
		response, err := namespace.client.RouterInspect(ctx)
		if err == nil {
			// Sometimes it takes a while for all the connected sites
			// to be reported. If we don't have them yet, keep waiting
			// until we get them, or until we time out.
			if response.Status.ConnectedSites.Direct >= expectedDirect &&
				response.Status.ConnectedSites.Indirect >= expectedIndirect {
				info(testName, "success.")
				return response.Status.ConnectedSites.Direct,
					response.Status.ConnectedSites.Indirect,
					nil
			}
		}
		if response != nil {
			info(testName, fmt.Sprintf("%d : (%d,%d) != (%d,%d)", attempts, response.Status.ConnectedSites.Direct, response.Status.ConnectedSites.Indirect, expectedDirect, expectedIndirect))
		}
		time.Sleep(time.Second)
		attempts--
	}
	return 0, 0, fmt.Errorf("timed out")
}

func check(t *testing.T, err error, test, msg string) {
	t.Helper()
	assert.Assert(t, err, "\n\n%sTest %s error : %s%s\n", red, test, msg, resetColor)
}

func info(test, msg string) {
	if verbose {
		fp(os.Stdout, "%s%s info: %s%s\n", green, test, msg, resetColor)
	}
}

func TestEdgeConnectivity(t *testing.T) {

	if !*clusterRun {
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(red), string(resetColor)))
		return
	}
	tests := []TopologyTest{
		// Test 1 ---------------------------------------------------
		{
			name:    "one-direct",
			diagram: []string{"edge  -->  interior"},
			namespaces: []*SkupperNamespace{
				&SkupperNamespace{
					name:   "test-1-edge", // Edge name must always contain the string "edge".
					isEdge: true,
				},
				&SkupperNamespace{
					name:   "test-1-interior",
					isEdge: false,
				},
			},
			connexions: []Connexion{
				{
					from: "test-1-edge",
					to:   "test-1-interior",
				},
			},
			direct:   1,
			indirect: 0,
		},
		// Test 2 ---------------------------------------------------
		{
			name: "two-direct-V",
			diagram: []string{"edge  -->  interior-1",
				"edge  -->  interior-2"},
			namespaces: []*SkupperNamespace{
				&SkupperNamespace{
					name:   "test-2-edge", // Edge name must always contain the string "edge".
					isEdge: true,
				},
				&SkupperNamespace{
					name:   "test-2-interior-1",
					isEdge: false,
				},
				&SkupperNamespace{
					name:   "test-2-interior-2",
					isEdge: false,
				},
			},
			connexions: []Connexion{
				{
					from: "test-2-edge",
					to:   "test-2-interior-1",
				},
				{
					from: "test-2-edge",
					to:   "test-2-interior-2",
				},
			},
			direct:   2,
			indirect: 0,
		},
		// Test 3 ---------------------------------------------------
		{
			name: "two-direct-triangle",
			diagram: []string{"edge  -->  interior-1",
				"edge  -->  interior-2",
				"interior-1  --> interior-2"},
			namespaces: []*SkupperNamespace{
				&SkupperNamespace{
					name:   "test-3-edge", // Edge name must always contain the string "edge".
					isEdge: true,
				},
				&SkupperNamespace{
					name:   "test-3-interior-1",
					isEdge: false,
				},
				&SkupperNamespace{
					name:   "test-3-interior-2",
					isEdge: false,
				},
			},
			connexions: []Connexion{
				{
					from: "test-3-edge",
					to:   "test-3-interior-1",
				},
				{
					from: "test-3-edge",
					to:   "test-3-interior-2",
				},
				{
					from: "test-3-interior-1",
					to:   "test-3-interior-2",
				},
			},
			direct:   2,
			indirect: 0,
		},
		// Test 4 ---------------------------------------------------
		{
			name: "three-direct",
			diagram: []string{"interior-1  -->  interior-2",
				"interior-2  -->  interior-3",
				"edge  -->  interior-1",
				"edge  -->  interior-2",
				"edge  -->  interior-3"},
			namespaces: []*SkupperNamespace{
				&SkupperNamespace{
					name:   "test-4-edge",
					isEdge: true,
				},
				&SkupperNamespace{
					name:   "test-4-interior-1",
					isEdge: false,
				},
				&SkupperNamespace{
					name:   "test-4-interior-2",
					isEdge: false,
				},
				&SkupperNamespace{
					name:   "test-4-interior-3",
					isEdge: false,
				},
			},
			connexions: []Connexion{
				{
					from: "test-4-interior-1",
					to:   "test-4-interior-2",
				},
				{
					from: "test-4-interior-2",
					to:   "test-4-interior-3",
				},
				{
					from: "test-4-edge",
					to:   "test-4-interior-1",
				},
				{
					from: "test-4-edge",
					to:   "test-4-interior-2",
				},
				{
					from: "test-4-edge",
					to:   "test-4-interior-3",
				},
			},
			direct:   3,
			indirect: 0,
		},
		// Test 5 ---------------------------------------------------
		{
			name: "one-direct-one-indirect",
			diagram: []string{"edge  -->  interior-1",
				"interior-1  -->  interior-2"},
			namespaces: []*SkupperNamespace{
				&SkupperNamespace{
					name:   "test-5-edge",
					isEdge: true,
				},
				&SkupperNamespace{
					name:   "test-5-interior-1",
					isEdge: false,
				},
				&SkupperNamespace{
					name:   "test-5-interior-2",
					isEdge: false,
				},
			},
			connexions: []Connexion{
				{
					from: "test-5-edge",
					to:   "test-5-interior-1",
				},
				{
					from: "test-5-interior-1",
					to:   "test-5-interior-2",
				},
			},
			direct:   1,
			indirect: 1,
		},
	}

	//--------------------------------------------
	// Run the tests.
	//--------------------------------------------
	for _, test := range tests {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var err error

		if !*clusterRun {
			t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", red, resetColor))
			return
		}

		if verbose {
			fp(os.Stdout, "\n\n%s%s\n\n", cyan, test.name)
			for _, s := range test.diagram {
				fp(os.Stdout, "\t%s\n", s)
			}
			fp(os.Stdout, "\n\tdirect: %d   indirect: %d\n", test.direct, test.indirect)
			fp(os.Stdout, "%s\n\n", resetColor)
		}

		// Here is where we will put the connection tokens, for
		// any of the namespaces in this test that require them.
		testPath := "./tmp/"
		os.Mkdir(testPath, 0755)
		defer os.RemoveAll(testPath)

		// Before we go though the list of namespaces to create them,
		// make a set of all namespace that will need to create connexion tokens.
		needToken := make(map[string]bool, 0)
		for _, cnx := range test.connexions {
			needToken[cnx.to] = true
		}

		//-----------------------------------------------------
		// Create and populate each namespace.
		//-----------------------------------------------------
		for _, namespace := range test.namespaces {
			namespace.client, err = NewClient(namespace.name, "", "")
			check(t, err, test.name, fmt.Sprintf("Can't create client for namespace |%s|.", namespace.name))
			info(test.name, fmt.Sprintf("Created client for namespace |%s|.", namespace.name))

			_, err = kube.NewNamespace(namespace.name, namespace.client.KubeClient)
			defer kube.DeleteNamespace(namespace.name, namespace.client.KubeClient)
			check(t, err, test.name, fmt.Sprintf("Can't create namespace |%s|.", namespace.name))
			info(test.name, fmt.Sprintf("Created namespace |%s|.", namespace.name))

			routerCreateOpts := types.SiteConfigSpec{
				SkupperName:       namespace.name,
				IsEdge:            namespace.isEdge,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          "",
				User:              "",
				Password:          "",
				ClusterLocal:      true,
			}
			siteConfig, err := namespace.client.SiteConfigCreate(context.Background(), routerCreateOpts)

			err = namespace.client.RouterCreate(ctx, *siteConfig)
			check(t, err, test.name, fmt.Sprintf("Can't create router for namespace |%s|", namespace.name))
			info(test.name, fmt.Sprintf("Created router for namespace |%s|", namespace.name))

			// If this namespace is in the set of all namespaces that
			// need to create a connexion token -- make it so.
			if _, ok := needToken[namespace.name]; ok {
				tokenName := "token-" + namespace.name
				namespace.secretFileName = testPath + tokenName + ".yaml"
				err = namespace.client.ConnectorTokenCreateFile(ctx, tokenName, namespace.secretFileName)
				check(t, err, test.name, fmt.Sprintf("Can't create connexion token for namespace |%s|.", namespace.name))
				info(test.name, fmt.Sprintf("Created connexion token for namespace |%s| at file |%s|.", namespace.name, namespace.secretFileName))

			}
		}

		//-----------------------------------------------------
		// Make all specified connexions.
		//-----------------------------------------------------
		for _, cnx := range test.connexions {

			fromNS := getNamespace(cnx.from, &test)
			toNS := getNamespace(cnx.to, &test)

			// Connect the from-client to the to-client.
			connectionName := cnx.from + cnx.to
			connectorName := "connector-for-" + connectionName
			_, err = fromNS.client.ConnectorCreateFromFile(ctx, toNS.secretFileName,
				types.ConnectorCreateOptions{
					Name:             connectorName,
					SkupperNamespace: fromNS.name,
					Cost:             1,
				})
			check(t, err, test.name, fmt.Sprintf("Can't create connector |%s|", connectionName))
			info(test.name, fmt.Sprintf("Created connector |%s|.", connectionName))
		}

		//-----------------------------------------------------
		// Finally, check the edge router to see if it has
		// the right connectivity.
		//-----------------------------------------------------
		edgeNS := getEdge(&test)

		direct, indirect, err := getConnectivity(test.name, edgeNS, ctx, 30, test.direct, test.indirect)
		check(t, err, test.name, "Can't get connectivity.")
		if direct != test.direct || indirect != test.indirect {
			assert.Check(t, false, "\n\n%sTest %s error : expected direct %d, indirect %d but got direct %d, indirect %d %s\n", red, test, test.direct, test.indirect, direct, indirect, resetColor)
		}
	}
}
