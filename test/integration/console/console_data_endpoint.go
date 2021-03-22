package console

import (
	"context"
	"fmt"
	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"github.com/skupperproject/skupper/test/utils/tools"
	"gotest.tools/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"os"
	"testing"
)

const consoleURL = "http://0.0.0.0:8080"
const consoleDataURL = "http://0.0.0.0:8080/DATA"

type BasicTestRunner struct {
	base.ClusterTestRunnerBase
}

type HttpRequestFromConsole struct {
	Requests int
	BytesIn  int
	BytesOut int
}

var (
	ctx, cancelFn = context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
)

// Create the deployment for the Frontend in public namespace
func CreateFrontendDeployment(t *testing.T, cluster *client.VanClient) {
	name := "hello-world-frontend"
	replicas := int32(1)
	dep := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    map[string]string{"app": name},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           "quay.io/skupper/" + name,
							ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Create(dep)
	assert.Assert(t, err)
}

// Create the deployment for the Backtend in public namespace
func CreateBackendDeployment(t *testing.T, cluster *client.VanClient) {
	name := "hello-world-backend"
	replicas := int32(1)
	dep := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    map[string]string{"app": name},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           "quay.io/skupper/" + name,
							ImagePullPolicy: corev1.PullIfNotPresent},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	// Deploying resource
	dep, err := cluster.KubeClient.AppsV1().Deployments(cluster.Namespace).Create(dep)
	assert.Assert(t, err)
}

func (r *BasicTestRunner) Setup(ctx context.Context, t *testing.T) {
	var err error

	log.Warn("Starting Setup procedure")

	var createOptsPublic = types.SiteConfigSpec{
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     true,
		AuthMode:          types.ConsoleAuthModeUnsecured,
		User:              "admin",
		Password:          "admin",
		Ingress:           types.IngressNoneString,
		Replicas:          1,
	}

	var createOptsPrivate = types.SiteConfigSpec{
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     true,
		AuthMode:          types.ConsoleAuthModeInternal,
		User:              "skupper-user",
		Password:          "skupper-pass",
		Ingress:           types.IngressNoneString,
		Replicas:          1,
	}

	// Get context for public
	publicCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	// Get context for private
	privateCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	// Create public namespace
	err = publicCluster.CreateNamespace()
	assert.Assert(t, err)

	// Create private namespace
	err = privateCluster.CreateNamespace()
	assert.Assert(t, err)

	// Create SiteConfig for public
	createOptsPublic.SkupperNamespace = publicCluster.Namespace
	siteConfig, err := publicCluster.VanClient.SiteConfigCreate(ctx, createOptsPublic)
	assert.Assert(t, err)
	err = publicCluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err)

	// Create SiteConfig for private
	createOptsPrivate.SkupperNamespace = privateCluster.Namespace
	siteConfig, err = privateCluster.VanClient.SiteConfigCreate(ctx, createOptsPrivate)
	assert.Assert(t, err)
	err = privateCluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err)

	// Create the connector token
	const secretFile = "/tmp/public_console_1_secret.yaml"
	err = publicCluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	assert.Assert(t, err)

	// Establish the connection
	var connectorCreateOpts = types.ConnectorCreateOptions{
		SkupperNamespace: privateCluster.Namespace,
	}
	_, err = privateCluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	assert.Assert(t, err)

	// Create the frontend Deployment
	CreateFrontendDeployment(t, publicCluster.VanClient)

	// Create the backend Deployment
	CreateBackendDeployment(t, privateCluster.VanClient)

	backsvc := types.ServiceInterface{
		Address:  "hello-world-backend",
		Protocol: "http",
		Port:     8080,
	}

	err = privateCluster.VanClient.ServiceInterfaceCreate(ctx, &backsvc)
	assert.Assert(t, err)

	err = privateCluster.VanClient.ServiceInterfaceBind(ctx, &backsvc, "deployment", "hello-world-backend", "http", 8080)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(publicCluster.Namespace, publicCluster.VanClient.KubeClient, "hello-world-backend")
	assert.Assert(t, err)

	frontsvc := types.ServiceInterface{
		Address:  "hello-world-frontend",
		Protocol: "http",
		Port:     8080,
	}

	err = publicCluster.VanClient.ServiceInterfaceCreate(ctx, &frontsvc)
	assert.Assert(t, err)

	err = publicCluster.VanClient.ServiceInterfaceBind(ctx, &frontsvc, "deployment", "hello-world-frontend", "http", 8080)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(publicCluster.Namespace, publicCluster.VanClient.KubeClient, "hello-world-frontend")
	assert.Assert(t, err)
}

// Remove the namespaces
func (r *BasicTestRunner) TearDown() {
	// Get context for public
	publicCluster, err := r.GetPublicContext(1)
	if err != nil {
		log.Warn("Unable to retrieve context for public Cluster")
		log.Warn("Check teardown manually")
	}

	// Get context for private
	privateCluster, err := r.GetPrivateContext(1)
	if err != nil {
		log.Warn("Unable to retrieve context for private Cluster")
		log.Warn("Check teardown manually")
	}

	// Remove public namespace
	err = publicCluster.DeleteNamespace()
	if err != nil {
		log.Warn("Unable to delete namespace public")
		log.Warn("Check teardown manually")
	}

	// Remove private namespace
	err = privateCluster.DeleteNamespace()
	if err != nil {
		log.Warn("Unable to delete namespace private")
		log.Warn("Check teardown manually")
	}
}

// Test if the Skupper console is available / accessible in Public cluster, unauthenticated
func (r *BasicTestRunner) testUnauthenticatedConsoleAvailable(t *testing.T) {

	const expectedReturnCode = 200

	// Get context for Public
	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	r.validateConsoleAcces(t, pubCluster, consoleURL, "", "", expectedReturnCode)
}

// Test if the Skupper console is available / accessible in Private cluster, authenticated
// with valid user/password
func (r *BasicTestRunner) testAuthenticatedConsoleAvailableValidUserPass(t *testing.T) {

	const expectedReturnCode = 200

	// Get context for Private
	privCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	username := "skupper-user"
	password := "skupper-pass"

	r.validateConsoleAcces(t, privCluster, consoleURL, username, password, expectedReturnCode)
}

// Test if the Skupper console is NOT available / accessible in Private cluster, authenticated
// with INVALID user/password
func (r *BasicTestRunner) testAuthenticatedConsoleAvailableInvalidUserPass(t *testing.T) {

	const expectedReturnCode = 401

	// Get context for Private
	privCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	username := "skupper-user"
	password := "not-real-pass"

	r.validateConsoleAcces(t, privCluster, consoleURL, username, password, expectedReturnCode)
}

func (r *BasicTestRunner) validateConsoleAcces(t *testing.T, cluster *base.ClusterContext, consoleURL string, userParam string, pwdParam string, expectedReturnCode int) {

	var CurlOptsForTest tools.CurlOpts
	if userParam != "" {
		CurlOptsForTest = tools.CurlOpts{Silent: true, Username: userParam, Password: pwdParam, Timeout: 10}
	} else {
		CurlOptsForTest = tools.CurlOpts{Silent: true, Timeout: 10}
	}

	res, err := tools.Curl(cluster.VanClient.KubeClient, cluster.VanClient.RestConfig, cluster.Namespace, "", consoleURL, CurlOptsForTest)
	assert.Assert(t, err)
	assert.Assert(t, res.StatusCode == expectedReturnCode)
}

// Test if the endpoint /DATA is accessible in Skupper Public/unauthenticated console
func (r *BasicTestRunner) testPublicDataEndpointAvailable(t *testing.T) {

	const expectedReturnCode = 200

	// Get context for Public
	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	r.validateConsoleAcces(t, pubCluster, consoleDataURL, "", "", expectedReturnCode)
}

// Test if the endpoint /DATA is accessible in Skupper Private/authenticated console
func (r *BasicTestRunner) testPrivateDataEndpointAvailable(t *testing.T) {

	const expectedReturnCode = 200

	// Get context for Private
	privCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	username := "skupper-user"
	password := "skupper-pass"

	r.validateConsoleAcces(t, privCluster, consoleDataURL, username, password, expectedReturnCode)
}

func getHttpRequestsNumbersFromConsole(cluster *base.ClusterContext, clientAddressFilter string, userParam string, passParam string) HttpRequestFromConsole {

	totConsoleData := HttpRequestFromConsole{0, 0, 0}

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
			if httpSvc.Address != clientAddressFilter {
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
func (r *BasicTestRunner) testDataEndpointOneRequest(t *testing.T) {

	r.testHttpFrontendEndpoint(t, 1)
}

// Test if request count in /DATA increases properly after five calls to the test frontend
func (r *BasicTestRunner) testDataEndpointFiveRequests(t *testing.T) {

	r.testHttpFrontendEndpoint(t, 5)
}

func (r *BasicTestRunner) testHttpFrontendEndpoint(t *testing.T, reqCount int) {
	const HelloWorldURL = "http://hello-world-frontend:8080"

	// Get context for Public
	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	// Get context for Private
	privCluster, err := r.GetPrivateContext(1)
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
