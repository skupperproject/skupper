// +build integration cli

package token

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/link"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/token"
	"gotest.tools/assert"
)

func TestToken(t *testing.T) {

	// First, validate if skupper binary is in the PATH, or skip test
	log.Printf("Running 'skupper --help' to determine if skupper binary is available")
	_, _, err := cli.RunSkupperCli([]string{"--help"})
	if err != nil {
		t.Skipf("skupper binary is not available")
	}

	needs := base.ClusterNeeds{
		NamespaceId:     "token",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	runner := &base.ClusterTestRunnerBase{}
	runner.BuildOrSkip(t, needs, nil)

	// getting public and private contexts
	pub, err := runner.GetPublicContext(1)
	assert.Assert(t, err)
	prv, err := runner.GetPrivateContext(1)
	assert.Assert(t, err)

	// creating namespaces
	assert.Assert(t, pub.CreateNamespace())
	assert.Assert(t, prv.CreateNamespace())

	// teardown once test completes
	tearDownFn := func() {
		log.Println("entering teardown")
		_ = pub.DeleteNamespace()
		_ = prv.DeleteNamespace()
	}
	defer tearDownFn()
	base.HandleInterruptSignal(t, func(t *testing.T) {
		tearDownFn()
	})

	// Creating a local directory for storing the token
	testPath := "./tmp"
	_ = os.Mkdir(testPath, 0755)
	tokenFile := fmt.Sprintf("%s/public-token-1.token.yaml", testPath)

	// Test scenarios to validate token types using skupper CLI
	scenarios := []cli.TestScenario{
		{
			Name: "initialize",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper init - interior mode
					&cli.InitTester{
						RouterMode: "interior",
					},
					// skupper status - verify initialized as interior
					&cli.StatusTester{
						RouterMode: "interior",
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper init - edge mode
					&cli.InitTester{
						RouterMode: "edge",
					},
					// skupper status - verify initialized as edge
					&cli.StatusTester{
						RouterMode: "edge",
					},
				}},
			},
		}, {
			// validating --token-type cert
			Name: "token-type-cert",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - verify token has been created
					&token.CreateTester{
						Name:     "public",
						FileName: tokenFile,
						Type:     token.TokenCert,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: tokenFile,
						Name:      "public",
					},
					// skupper link status - to assert sites are connected
					&link.StatusTester{
						Name:   "public",
						Active: true,
					},
					// skupper link delete - to remove it
					&link.DeleteTester{
						Name: "public",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
					// skupper link create - using same connection token
					&link.CreateTester{
						TokenFile: tokenFile,
						Name:      "public",
					},
					// skupper link status - to assert connection token is reusable
					&link.StatusTester{
						Name:   "public",
						Active: true,
					},
					// skupper link delete - to remove it
					&link.DeleteTester{
						Name: "public",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
				}},
			},
		}, {
			// validate --token-type claim (default)
			Name: "token-type-claim",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - verify token has been created
					&token.CreateTester{
						FileName: tokenFile,
						Type:     token.TokenClaim,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: tokenFile,
					},
					// skupper link status - to assert sites are connected
					&link.StatusTester{
						Active: true,
					},
					// skupper link delete - to remove it
					&link.DeleteTester{
						Name: "conn1",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
					// skupper link create - using same token claim
					&link.CreateTester{
						TokenFile: tokenFile,
					},
					// skupper link status - connection should fail as claim can only be used once by default
					&link.StatusTester{
						Active:  false,
						Failure: link.ClaimInvalid,
					},
					&link.DeleteTester{
						Name: "conn1",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
				}},
			},
		}, {
			// validate --token-type claim using all supported flags (instead of default values)
			Name: "token-type-claim-all-flags",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - verify token has been created
					&token.CreateTester{
						Name:     "public",
						FileName: tokenFile,
						Expiry:   "10m",
						Password: "password",
						Type:     token.TokenClaim,
						Uses:     "2",
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: tokenFile,
					},
					// skupper link status - to assert sites are connected
					&link.StatusTester{
						Active: true,
					},
					// skupper link delete - to remove it
					&link.DeleteTester{
						Name: "conn1",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
					// skupper link create - using same connection token
					&link.CreateTester{
						TokenFile: tokenFile,
					},
					// skupper link status - to assert claim could be used twice
					&link.StatusTester{
						Active: true,
					},
					&link.DeleteTester{
						Name: "conn1",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
				}},
			},
		}, {
			Name: "token-type-claim-expired",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - verify token has been created
					&token.CreateTester{
						Name:      "public",
						FileName:  tokenFile,
						Expiry:    "10s",
						Type:      token.TokenClaim,
						PostDelay: 15 * time.Second,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: tokenFile,
					},
					// skupper link status - to assert connection failed as claim expired
					&link.StatusTester{
						Active:  false,
						Failure: link.ClaimInvalid,
					},
					// skupper link delete - to remove it
					&link.DeleteTester{
						Name: "conn1",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
				}},
			},
		}, {
			Name: "revoke-access",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - verify token has been created
					&token.CreateTester{
						FileName: tokenFile,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: tokenFile,
					},
					// skupper link status - to assert sites are connected
					&link.StatusTester{
						Active: true,
					},
				}},
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper revoke-access - to revoke access to all emitted certificates
					&cli.RevokeAccessTester{
						ExpectClaimRecordsDeleted: true,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link status - to assert sites are no longer connected
					&link.StatusTester{
						Active: false,
					},
					// skupper link delete - to remove it
					&link.DeleteTester{
						Name: "conn1",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
				}},
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - assert that a new token has been created
					&token.CreateTester{
						FileName: tokenFile,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: tokenFile,
					},
					// skupper link status - to assert sites are connected using certificate emitted by new CA
					&link.StatusTester{
						Active: true,
					},
					// skupper link delete - to remove it
					&link.DeleteTester{
						Name: "conn1",
					},
					// skupper status - to assert sites are no longer connected
					&cli.StatusTester{
						RouterMode: "edge",
					},
				}},
			},
		},
	}

	// Running the scenarios
	for _, scenario := range scenarios {
		var stdout, stderr string
		passed := t.Run(scenario.Name, func(t *testing.T) {
			stdout, stderr, err = cli.RunScenario(scenario)
			assert.Assert(t, err)
		})
		if !passed {
			log.Printf("%s has failed, exiting", scenario.Name)
			log.Printf("STDOUT:\n%s", stdout)
			log.Printf("STDERR:\n%s", stderr)
			break
		}
	}

}
