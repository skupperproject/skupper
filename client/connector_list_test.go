package client

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
	"os"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestBreakDownConnectionsList(t *testing.T) {
	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
		return
	}

	testcases := []struct {
		namespace     string
		doc           string
		createConn    bool
		connName      string
		incomingSite1 int
		outgoingSite1 int
		incomingSite2 int
		outgoingSite2 int
	}{
		{
			namespace:     "van-link-status-1",
			doc:           "Should return the incoming and outgoing links.",
			createConn:    true,
			connName:      "link1",
			incomingSite1: 1,
			outgoingSite1: 0,
			incomingSite2: 0,
			outgoingSite2: 1,
		},
		{
			namespace:     "van-link-status-2",
			doc:           "Should not return the incoming nor outgoing links if they don't exist.",
			createConn:    false,
			connName:      "",
			incomingSite1: 0,
			outgoingSite1: 0,
			incomingSite2: 0,
			outgoingSite2: 0,
		},
	}

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	for _, c := range testcases {
		fmt.Println("Test case: " + c.doc)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create and set up the two namespaces that we will be using.
		tokenCreatorNamespace := c.namespace + "-token-creator"
		tokenUserNamespace := c.namespace + "-token-user"
		tokenCreatorClient, tokenUserClient, err := setupTwoNamespaces(t, ctx, tokenCreatorNamespace, tokenUserNamespace)
		assert.Assert(t, err, "Can't set up namespaces")
		defer kube.DeleteNamespace(tokenCreatorNamespace, tokenCreatorClient.KubeClient)
		defer kube.DeleteNamespace(tokenUserNamespace, tokenUserClient.KubeClient)

		if c.createConn {
			err = tokenCreatorClient.ConnectorTokenCreateFile(ctx, c.connName, testPath+c.connName+".yaml")
			assert.Check(t, err, "Unable to create connector token "+c.connName)

			_, err = tokenUserClient.ConnectorCreateFromFile(ctx, testPath+c.connName+".yaml", types.ConnectorCreateOptions{
				Name:             c.connName,
				SkupperNamespace: tokenUserNamespace,
				Cost:             1,
			})
			assert.Check(t, err, "Unable to create connector for "+c.connName)

		}

		// wait for connection
		err = waitUntilConnectionsAreUpdated(tokenUserClient, c.createConn)
		assert.Assert(t, err)

		resultsSite1, _ := tokenCreatorClient.BreakDownConnectionsList()

		assert.DeepEqual(t, resultsSite1, map[string]int{"in": c.incomingSite1, "out": c.outgoingSite1})

		resultsSite2, _ := tokenUserClient.BreakDownConnectionsList()

		assert.DeepEqual(t, resultsSite2, map[string]int{"in": c.incomingSite2, "out": c.outgoingSite2})

	}

}

func waitUntilConnectionsAreUpdated(cli *VanClient, createdConn bool) error {
	var err error

	err = utils.Retry(time.Second*5, 8, func() (bool, error) {
		connections, err := qdr.GetConnections(cli.Namespace, cli.KubeClient, cli.RestConfig)
		if err != nil {
			return false, nil
		}
		for _, c := range connections {
			if createdConn && (c.Role == "inter-router" || c.Role == "edge") {
				return true, nil
			}
			if !createdConn && c.Role == "normal" {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}
