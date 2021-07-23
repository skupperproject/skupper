//+build integration

package console

import (
	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"

	"os"
	"testing"
)

// Flag parsing - Main step in package
func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestDataEndpoint(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "data-endpoint",
		PublicClusters:  1,
		PrivateClusters: 1,
	}

	testRunner := &BasicTestRunner{}
	if err := testRunner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err := testRunner.Build(needs, nil)
	assert.Assert(t, err)
	base.HandleInterruptSignal(t, func(t *testing.T) {
		cancelFn()
		testRunner.TearDown()
	})

	// Setup skupper Infra
	testRunner.Setup(ctx, t)

	// Remove test resources / Infra after test run
	defer testRunner.TearDown()

	// Test set

	// Test if the Skupper console is available / accessible in Public cluster, unauthenticated
	t.Run("test-unauthenticated-console-available", testRunner.testUnauthenticatedConsoleAvailable)

	// Test if the Skupper console is available / accessible in Private cluster, authenticated
	t.Run("test-authenticated-console-available-valid-user-pass", testRunner.testAuthenticatedConsoleAvailableValidUserPass)

	// Test if the Skupper console is NOT available / accessible in Private cluster, using an invalid user/password
	t.Run("test-authenticated-console-available-invalid-user-pass", testRunner.testAuthenticatedConsoleAvailableInvalidUserPass)

	// Test if the endpoint /DATA is accessible in Skupper Public/unauthenticated console
	t.Run("test-public-data-endpoint-available", testRunner.testPublicDataEndpointAvailable)

	// Test if the endpoint /DATA is accessible in Skupper Private/authenticated console
	t.Run("test-private-data-endpoint-available", testRunner.testPrivateDataEndpointAvailable)

	// Test if values in /DATA increases after one call to the test frontend
	t.Run("test-data-endpoint-one-request", testRunner.testDataEndpointOneRequest)

	// Test if request count in /DATA increases properly after five calls to the test frontend
	t.Run("test-data-endpoint-five-requests", testRunner.testDataEndpointFiveRequests)
}
