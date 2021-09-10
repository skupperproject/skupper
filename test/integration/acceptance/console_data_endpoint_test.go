// +build integration acceptance console

package acceptance

import (
	"context"
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/test/integration/acceptance/console"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/tools"
	"gotest.tools/assert"
)

const consoleURL = "https://0.0.0.0:8080"
const consoleDataURL = "https://0.0.0.0:8080/DATA"

func TestDataEndpoint(t *testing.T) {
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()

	// Setup deployments
	defer console.TearDown(t, testRunner)
	console.Setup(ctx, t, testRunner)

	// Test set

	// Test if the Skupper console is available / accessible in Public cluster, unauthenticated
	t.Run("test-unauthenticated-console-available", testUnauthenticatedConsoleAvailable)

	// Test if the Skupper console is available / accessible in Private cluster, authenticated
	t.Run("test-authenticated-console-available-valid-user-pass", testAuthenticatedConsoleAvailableValidUserPass)

	// Test if the Skupper console is NOT available / accessible in Private cluster, using an invalid user/password
	t.Run("test-authenticated-console-available-invalid-user-pass", testAuthenticatedConsoleAvailableInvalidUserPass)

	// Test if the endpoint /DATA is accessible in Skupper Public/unauthenticated console
	t.Run("test-public-data-endpoint-available", testPublicDataEndpointAvailable)

	// Test if the endpoint /DATA is accessible in Skupper Private/authenticated console
	t.Run("test-private-data-endpoint-available", testPrivateDataEndpointAvailable)

	// Test if values in /DATA increases after one call to the test frontend
	t.Run("test-data-endpoint-one-request", testDataEndpointOneRequest)

	// Test if request count in /DATA increases properly after five calls to the test frontend
	t.Run("test-data-endpoint-five-requests", testDataEndpointFiveRequests)
}

// Test if the Skupper console is available / accessible in Public cluster, authenticated
// with valid user/password
func testAuthenticatedConsoleAvailableValidUserPass(t *testing.T) {
	const expectedReturnCode = 200

	// Get context for Public
	pubCluster, err := testRunner.GetPublicContext(1)
	assert.Assert(t, err)

	testConsoleAccess(t, pubCluster, consoleURL, "admin", "admin", expectedReturnCode)
}

// Test if the Skupper console is available / accessible in Private cluster, unauthenticated
func testUnauthenticatedConsoleAvailable(t *testing.T) {
	t.Run("test-unauthenticated-console-available", func(t *testing.T) {
		const expectedReturnCode = 200

		// Get context for Private
		privCluster, err := testRunner.GetPrivateContext(1)
		assert.Assert(t, err)

		testConsoleAccess(t, privCluster, consoleURL, "", "", expectedReturnCode)
	})
}

// Test if the Skupper console is NOT available / accessible in Public cluster, authenticated
// with INVALID user/password
func testAuthenticatedConsoleAvailableInvalidUserPass(t *testing.T) {

	const expectedReturnCode = 401

	// Get context for Private
	pubCluster, err := testRunner.GetPublicContext(1)
	assert.Assert(t, err)

	username := "skupper-user"
	password := "not-real-pass"

	testConsoleAccess(t, pubCluster, consoleURL, username, password, expectedReturnCode)
}

func testConsoleAccess(t *testing.T, cluster *base.ClusterContext, consoleURL string, userParam string, pwdParam string, expectedReturnCode int) {

	var CurlOptsForTest tools.CurlOpts
	if userParam != "" {
		CurlOptsForTest = tools.CurlOpts{Silent: true, Insecure: true, Username: userParam, Password: pwdParam, Timeout: 10}
	} else {
		CurlOptsForTest = tools.CurlOpts{Silent: true, Insecure: true, Timeout: 10}
	}

	res, err := tools.Curl(cluster.VanClient.KubeClient, cluster.VanClient.RestConfig, cluster.Namespace, "", consoleURL, CurlOptsForTest)
	assert.Assert(t, err)
	assert.Assert(t, res.StatusCode == expectedReturnCode, "status = %d - expected = %d", res.StatusCode, expectedReturnCode)
}

// Test if the endpoint /DATA is accessible in Skupper Public/unauthenticated console
func testPublicDataEndpointAvailable(t *testing.T) {
	const expectedReturnCode = 200

	// Get context for Public
	pubCluster, err := testRunner.GetPublicContext(1)
	assert.Assert(t, err)

	testConsoleAccess(t, pubCluster, consoleDataURL, "admin", "admin", expectedReturnCode)
}

// Test if the endpoint /DATA is accessible in Skupper Private/authenticated console
func testPrivateDataEndpointAvailable(t *testing.T) {

	const expectedReturnCode = 200

	// Get context for Private
	privCluster, err := testRunner.GetPrivateContext(1)
	assert.Assert(t, err)

	testConsoleAccess(t, privCluster, consoleDataURL, "", "", expectedReturnCode)
}

func getHttpRequestsNumbersFromConsole(cluster *base.ClusterContext, clientAddressFilter string, userParam string, passParam string) console.HttpRequestFromConsole {

	totConsoleData := console.HttpRequestFromConsole{
		Requests: 0,
		BytesIn:  0,
		BytesOut: 0,
	}

	consoleData, err := base.GetConsoleData(cluster, userParam, passParam)
	if err != nil {
		fmt.Println("[getHttpRequestsNumbersFromConsole] - Unable to retrieve ConsoleData")
		return totConsoleData
	}

	// If there is no requests or bytes available yet
	if consoleData.Services == nil || len(consoleData.Services) == 0 {
		fmt.Println("[getHttpRequestsNumbersFromConsole] - No Services available in ConsoleData")
		return totConsoleData
	}

	// There is data already available
	for _, svc := range consoleData.Services {

		httpSvc, ok := svc.(data.HttpService)
		if !ok {
			continue
		}

		if clientAddressFilter != "" {
			if httpSvc.AddressUnqualified() != clientAddressFilter {
				continue
			}
		}

		// If we have any request received, get the value
		if len(svc.(data.HttpService).RequestsReceived) > 0 {
			for _, reqsReceived := range svc.(data.HttpService).RequestsReceived {

				for _, reqByClient := range reqsReceived.ByClient {
					totConsoleData.Requests += reqByClient.Requests
					totConsoleData.BytesIn += reqByClient.BytesIn
					totConsoleData.BytesOut += reqByClient.BytesOut
					break
				}
			}
		}
	}
	return totConsoleData
}

// Test if values in /DATA increases after one call to the test frontend
func testDataEndpointOneRequest(t *testing.T) {
	testHttpFrontendEndpoint(t, 1)
}

// Test if request count in /DATA increases properly after five calls to the test frontend
func testDataEndpointFiveRequests(t *testing.T) {
	testHttpFrontendEndpoint(t, 5)
}

func testHttpFrontendEndpoint(t *testing.T, reqCount int) {
	const HelloWorldURL = "http://hello-world-frontend:8080"

	// Get context for Public
	pubCluster, err := testRunner.GetPublicContext(1)
	assert.Assert(t, err)

	// Get context for Private
	privCluster, err := testRunner.GetPrivateContext(1)
	assert.Assert(t, err)

	// Get current number of requests from both clusters
	iniPub := getHttpRequestsNumbersFromConsole(pubCluster, "hello-world-frontend", "admin", "admin")
	iniPriv := getHttpRequestsNumbersFromConsole(privCluster, "hello-world-frontend", "skupper-user", "skupper-pass")

	// Send the requests to the service
	for reqs := 0; reqs < reqCount; reqs++ {
		res, err := tools.Curl(pubCluster.VanClient.KubeClient, pubCluster.VanClient.RestConfig, pubCluster.Namespace, "", HelloWorldURL, tools.CurlOpts{Silent: true, Timeout: 10})
		assert.Assert(t, err)
		assert.Assert(t, res.StatusCode == 200)
	}

	// Get current number of requests from both clusters
	endPub := getHttpRequestsNumbersFromConsole(pubCluster, "hello-world-frontend", "admin", "admin")
	endPriv := getHttpRequestsNumbersFromConsole(privCluster, "hello-world-frontend", "skupper-user", "skupper-pass")

	// Check if Request Received numbers has increased in Public
	assert.Assert(t, endPub.Requests == iniPub.Requests+reqCount, "Requests has not increased 5 units in the Public cluster - IniReqs |%d| - EndReqs |%d| - json |%s|", iniPub.Requests, endPub.Requests)

	// Check if Request Received numbers has increased in Private
	assert.Assert(t, endPriv.Requests == iniPriv.Requests+reqCount, "Requests has not increased 5 units in the Private cluster - IniReqs |%d| - EndReqs |%d|", iniPub.Requests, endPub.Requests)

	// Check if BytesIn has increased in Public
	assert.Assert(t, endPub.BytesIn > iniPub.BytesIn, "BytesIn has not increased in the Public cluster")

	// Check if BytesIn has increased in Private
	assert.Assert(t, endPriv.BytesIn > iniPriv.BytesIn, "BytesIn has not increased in the Private cluster")

	// Check if BytesOut has increased in Public
	assert.Assert(t, endPub.BytesOut > iniPub.BytesOut, "BytesOut has not increased in the Public cluster")

	// Check if BytesOut has increased in Private
	assert.Assert(t, endPriv.BytesOut > iniPriv.BytesOut, "BytesOut has not increased in the Private cluster")
}
