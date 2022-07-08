package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/spf13/cobra"
	"gotest.tools/assert"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
)

type testCase struct {
	doc             string
	args            []string
	expectedCapture string
	expectedOutput  string
	expectedError   string
	outputRegExp    string
	realCluster     bool
	createConn      bool
}

func executeCommand(cmd *cobra.Command, args ...string) (cmdOut string, err error) {
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetErr(bufOut)
	cmd.SetArgs(args)

	cmd.SilenceErrors = true
	err = cmd.ExecuteContext(context.Background())
	return bufOut.String(), err
}

func checkStringContains(t *testing.T, got, expected string) {
	if !strings.Contains(got, expected) {
		t.Errorf("Expected to contain: \n %v\nGot:\n %v\n", expected, got)
	}
}

func checkStringOmits(t *testing.T, got, expected string) {
	if strings.Contains(got, expected) {
		t.Errorf("Expected to not contain: \n %v\nGot: %v", expected, got)
	}
}

func checkRegularExpression(t *testing.T, got, expected string) {
	if match, _ := regexp.MatchString(expected, got); !match {
		t.Errorf("Expected to match: \n %v\nGot: %v", expected, got)
	}
}

func newMockClient(namespace string) *client.VanClient {
	return &client.VanClient{
		Namespace:  namespace,
		KubeClient: fake.NewSimpleClientset(),
	}
}

func testClient(cmd *cobra.Command, args []string) {
	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}
}

func skupperInit(t *testing.T, args ...string) {
	initCmd := NewCmdInit(testClient)
	silenceCobra(initCmd)

	rescueStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	_, err := executeCommand(initCmd, args...)

	w.Close()
	os.Stdout = rescueStdout
	assert.Assert(t, err)
}

func skupperExpose(t *testing.T, args ...string) {
	exposeCmd := NewCmdExpose(testClient)
	silenceCobra(exposeCmd)

	rescueStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	_, err := executeCommand(exposeCmd, args...)

	w.Close()
	os.Stdout = rescueStdout
	assert.Assert(t, err)
}

func testCommand(t *testing.T, cmd *cobra.Command, testName string, expectedError string, expectedCapture string, expectedOutput string, outputRegExp string, args ...string) {
	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdOutput, err := executeCommand(cmd, args...)

	w.Close()
	stdOutput, _ := ioutil.ReadAll(r)
	os.Stdout = rescueStdout

	lines := strings.Split(string(stdOutput), "\n")

	if expectedError != "" {
		assert.Assert(t, err != nil)
		checkStringContains(t, err.Error(), expectedError)
	} else {
		assert.Check(t, err, testName)
	}
	if expectedCapture != "" {
		checkStringContains(t, lines[0], expectedCapture)
	} else if outputRegExp == "" {
		assert.Equal(t, string(stdOutput), "")
	}
	if expectedOutput != "" {
		checkStringContains(t, cmdOutput, expectedOutput)
	} else {
		assert.Equal(t, cmdOutput, "")
	}

	if outputRegExp != "" {
		checkRegularExpression(t, string(stdOutput), outputRegExp)
	}
}

var depReplicas int32 = 1
var tcpDeployment *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "tcp-go-echo",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &depReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "tcp-go-echo"},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "tcp-go-echo",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "tcp-go-echo",
						Image:           "quay.io/skupper/tcp-go-echo",
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		},
	},
}

var ssReplicas int32 = 2
var tcpStatefulSet *appsv1.StatefulSet = &appsv1.StatefulSet{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "tcp-go-echo-ss",
	},
	Spec: appsv1.StatefulSetSpec{
		Replicas:    &ssReplicas,
		ServiceName: "tcp-go-echo-ss",
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "tcp-go-echo-ss"},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "tcp-go-echo-ss",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "tcp-go-echo",
						Image:           "quay.io/skupper/tcp-go-echo",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name:          "tcp-go-echo",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 9090,
							},
						},
					},
				},
			},
		},
	},
}
var statefulSetService *corev1.Service = &corev1.Service{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "tcp-go-echo-ss",
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			"application": "tcp-go-echo-ss",
		},
		Ports: []corev1.ServicePort{
			corev1.ServicePort{
				Name: "tcp-go-echo",
				Port: int32(9090),
			},
		},
	},
}

func TestInitInteriorWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "init-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Setup a router and other supporting objects to provide a functional skupper",
			expectedError:   "",
			realCluster:     false,
		},
	}

	namespace = "init-interior-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdInit(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestDeleteWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "delete-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "delete will delete any skupper related objects from the namespace",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "delete-test2",
			args:            []string{},
			expectedCapture: "Skupper is now removed from",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-delete-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdDelete(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestConnectionTokenWithEdgeCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "connection-token-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Create a connection token.  The 'connect' command uses the token to establish a connection from a remote Skupper site.",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "connection-token-test2",
			args:            []string{},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "accepts 1 arg(s), received 0",
			realCluster:     false,
		},
		{
			doc:             "connection-token-test3",
			args:            []string{"/tmp/foo.yaml"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Failed to create token: Edge configuration cannot accept connections",
			realCluster:     true,
		},
	}

	namespace = "cmd-connection-token-edge-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)

	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdConnectionToken(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestConnectionTokenWithInteriorCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "connection-token-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Create a connection token.  The 'connect' command uses the token to establish a connection from a remote Skupper site.",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "connection-token-test2",
			args:            []string{},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "accepts 1 arg(s), received 0",
			realCluster:     false,
		},
		{
			doc:             "connection-token-test3",
			args:            []string{"/tmp/foo.yaml"},
			expectedCapture: "Token written to /tmp/foo.yaml",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-connection-token-interior-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)

	}
	skupperInit(t, []string{"--cluster-local"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdConnectionToken(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestConnectWithEdgeCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "connect-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Connect this skupper installation to that which issued the specified connectionToken",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "connect-test2",
			args:            []string{"/tmp/foo.yaml", "--connection-name", "edge-conn1"},
			expectedCapture: "Site configured to link to",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-connect-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdConnect(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestConnectWithInteriorCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "connect-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Connect this skupper installation to that which issued the specified connectionToken",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "connect-test2",
			args:            []string{"/tmp/foo.yaml", "--connection-name", "interior-conn1"},
			expectedCapture: "Site configured to link to",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-connect-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--cluster-local"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdConnect(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestDisconnectWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "disconnect-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Remove specified connection",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "disconnect-test2",
			args:            []string{"link1"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "No such link \"link1\"",
			realCluster:     true,
		},
		{
			doc:             "disconnect-test3",
			args:            []string{"link1"},
			expectedCapture: "Link 'link1' has been removed",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
			createConn:      true,
		},
	}

	namespace = "cmd-disconnect-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		if tc.createConn {
			if c, ok := cli.(*client.VanClient); ok {
				token := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "link1",
						Labels: map[string]string{
							types.SkupperTypeQualifier: types.TypeToken,
						},
					},
				}
				_, err := c.KubeClient.CoreV1().Secrets(namespace).Create(token)
				assert.Check(t, err)
			}
		}
		cmd := NewCmdDisconnect(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestListConnectorsWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "list-connectors-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "List configured outgoing connections",
			expectedError:   "",
			realCluster:     false,
			createConn:      false,
		},
		{
			doc:             "list-connectors-test2",
			args:            []string{},
			expectedCapture: "There are no connectors defined.",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
			createConn:      false,
		},
		{
			doc:             "list-connectors-test3",
			args:            []string{},
			expectedCapture: "Connectors:",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
			createConn:      true,
		},
	}

	namespace = "cmd-list-connectors-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		// create a connection to list
		if tc.createConn {
			connectCmd := NewCmdConnect(testClient)
			silenceCobra(connectCmd)
			testCommand(t, connectCmd, tc.doc, "", "Site configured to link to", "", "", []string{"/tmp/foo.yaml"}...)
		}

		cmd := NewCmdListConnectors(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestCheckConnectionWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "check-connection-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Check whether a connection to another Skupper site is active",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "check-connection-test2",
			args:            []string{"all"},
			expectedCapture: "There are no links configured or active",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "check-connection-test3",
			args:            []string{"link1"},
			expectedCapture: "No such link",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "check-connection-test4",
			args:            []string{"link1"},
			expectedCapture: "Link link1 not active",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
			createConn:      true,
		},
		{
			doc:             "check-connection-test5",
			args:            []string{"all"},
			expectedCapture: "Link link1 not active",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-check-connection-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		if tc.createConn {
			cmd := NewCmdConnect(testClient)
			silenceCobra(cmd)
			testCommand(t, cmd, tc.doc, "", "Site configured to link to", "", "", []string{"/tmp/foo.yaml"}...)
		}

		cmd := NewCmdCheckConnection(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestStatusWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "status-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Report the status of the current Skupper site",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "status-test2",
			args:            []string{},
			expectedCapture: "Skupper is enabled for namespace",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-status-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdStatus(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestExposeWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "expose-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Expose a set of pods through a Skupper address",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "expose-test2",
			args:            []string{},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
			realCluster:     false,
		},
		{
			doc:             "expose-test3",
			args:            []string{"deployment"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
			realCluster:     false,
		},
		{
			doc:             "expose-test4",
			args:            []string{"deployment", "tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to create skupper service:",
			realCluster:     false,
		},
		{
			doc:             "expose-test5",
			args:            []string{"deployment/tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to create skupper service:",
			realCluster:     false,
		},
		{
			doc:             "expose-test6",
			args:            []string{"deployent", "tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "target type must be one of: [deployment, statefulset, pods, service]",
			realCluster:     false,
		},
		{
			doc:             "expose-test7",
			args:            []string{"pods", "tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to create skupper service: VAN service interfaces for pods not yet implemented",
			realCluster:     true,
		},
		{
			doc:             "expose-test8",
			args:            []string{"deployment", "tcp-go-echo"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to create skupper service: Service port required and cannot be deduced.",
			realCluster:     true,
		},
		{
			doc:             "expose-test9",
			args:            []string{"deployment", "tcp-go-echo", "--port", "9090"},
			expectedCapture: "deployment tcp-go-echo exposed as tcp-go-echo",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "expose-test10",
			args:            []string{"deployment", "tcp-go-echo-invalid", "--port", "1234567890"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to create skupper service: Port 1234567890 is outside valid range.",
			realCluster:     true,
		},
		{
			doc:             "expose-test11",
			args:            []string{"deployment", "tcp-go-echo", "--barney"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "unknown flag: --barney",
			realCluster:     true,
		},
		{
			doc:             "expose-test12",
			args:            []string{"deployment", "tcp-not-deployed", "--headless"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "The headless option is only supported for statefulsets",
			realCluster:     true,
		},
		{
			doc:             "expose-test13",
			args:            []string{"statefulset", "tcp-go-echo-ss"},
			expectedCapture: "statefulset tcp-go-echo-ss exposed as tcp-go-echo",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "expose-test14",
			args:            []string{"statefulset", "tcp-go-echo-ss", "--headless", "--address", "tcp-go-echo-ss"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Service already exposed, cannot reconfigure as headless",
			realCluster:     true,
		},
		{
			doc:             "expose-test15",
			args:            []string{"service", "tcp-go-echo"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "--address option is required for target type 'service'",
			realCluster:     true,
		},
		{
			doc:             "expose-test16",
			args:            []string{"service", "tcp-go-echo", "--port", "9090", "--address", "tcp-go-echo-dup"},
			expectedCapture: "service tcp-go-echo exposed as tcp-go-echo-dup",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-expose-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)

		// create a target deployment as pre-condition
		deployments := c.KubeClient.AppsV1().Deployments(namespace)
		statefulSets := c.KubeClient.AppsV1().StatefulSets(namespace)
		services := c.KubeClient.CoreV1().Services(namespace)
		_, err = deployments.Create(tcpDeployment)
		assert.Assert(t, err)
		_, err = statefulSets.Create(tcpStatefulSet)
		assert.Assert(t, err)
		_, err = services.Create(statefulSetService)
		assert.Assert(t, err)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		t.Run(tc.doc, func(t *testing.T) {
			if tc.realCluster && !*clusterRun {
				return
			}
			if *clusterRun && len(tc.args) > 0 && tc.args[0] == "service" {
				c := cli.(*client.VanClient)
				_, _ = kube.WaitServiceExists(tc.args[1], cli.GetNamespace(), c.KubeClient, time.Second*60, time.Second*5)
			}
			cmd := NewCmdExpose(testClient)
			silenceCobra(cmd)
			testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
		})
	}
}

func TestUnexposeWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "unexpose-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Unexpose a set of pods previously exposed through a Skupper address",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test2",
			args:            []string{},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test3",
			args:            []string{"deployment"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test4",
			args:            []string{"deployment", "tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to unbind skupper service:",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test4",
			args:            []string{"deployment/tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to unbind skupper service:",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test5",
			args:            []string{"deployent", "tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "target type must be one of: [deployment, statefulset, pods, service]",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test6",
			args:            []string{"pods", "tcp-not-deployed"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Unable to unbind skupper service: Target type for service interface not yet implemented",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test7",
			args:            []string{"deployment", "tcp-not-deployed", "extraArg"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "illegal argument: extraArg",
			realCluster:     false,
		},
		{
			doc:             "unexpose-test8",
			args:            []string{"deployment", "tcp-not-deployed", "extraArg", "extraExtraArg"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "illegal argument: extraArg",
			realCluster:     false,
		},
	}

	namespace = "cmd-unexpose-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}

		cmd := NewCmdUnexpose(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}

}

func TestListExposedWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "list-exposed-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "List services exposed over the service network",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "list-exposed-test2",
			args:            []string{},
			expectedCapture: "Services exposed through Skupper",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-list-exposed-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)

		// create a target deployment as pre-condition
		deployments := c.KubeClient.AppsV1().Deployments(namespace)
		_, err = deployments.Create(tcpDeployment)
		assert.Assert(t, err)
	}

	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	if *clusterRun {
		exposeCmd := NewCmdExpose(testClient)
		silenceCobra(exposeCmd)
		testCommand(t, exposeCmd, "cmd-list-exposed-cluster-test", "", "deployment tcp-go-echo exposed as tcp-go-echo", "", "", []string{"deployment", "tcp-go-echo", "--port", "9090"}...)
	}

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdListExposed(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestCreateServiceWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "create-service-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Create a skupper service",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "create-service-test2",
			args:            []string{"tcp-go-echo-a", "9090"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "create-service-test3",
			args:            []string{"tcp-go-echo-b", "909x"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "909x is not a valid port",
			realCluster:     false,
		},
		{
			doc:             "create-service-test4",
			args:            []string{"tcp-go-echo-c:9090"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},

		{
			doc:             "create-service-test5",
			args:            []string{"tcp-go-echo-c:909x"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "909x is not a valid port",
			realCluster:     false,
		},
	}

	namespace = "cmd-create-service-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdCreateService(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestDeleteServiceWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "delete-service-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Delete a skupper service",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "delete-service-test2",
			args:            []string{"tcp-go-echo-a"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Service tcp-go-echo-a not defined",
			realCluster:     true,
		},
		{
			doc:             "delete-service-test3",
			args:            []string{"tcp-go-echo-b"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-delete-service-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	if *clusterRun {
		createCmd := NewCmdCreateService(testClient)
		silenceCobra(createCmd)
		testCommand(t, createCmd, "", "", "", "", "", []string{"tcp-go-echo-b:9090"}...)
	}

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdDeleteService(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestBindWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "bind-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Bind a target to a service",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "bind-test2",
			args:            []string{"tcp-go-echo-a", "deployment", "tcp-go-echo-a"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Service tcp-go-echo-a not found",
			realCluster:     true,
		},
		{
			doc:             "bind-test3",
			args:            []string{"tcp-go-echo", "deployment", "tcp-go-echo"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "bind-test4",
			args:            []string{"tcp-go-echo", "deployment", "tcp-go-echo3", "--protocol", "sctp"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "sctp is not a valid protocol. Choose 'tcp', 'http' or 'http2'",
			realCluster:     true,
		},
	}

	namespace = "cmd-bind-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)

		// create a target deployment as pre-condition
		deployments := c.KubeClient.AppsV1().Deployments(namespace)
		_, err = deployments.Create(tcpDeployment)
		assert.Assert(t, err)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	if *clusterRun {
		createCmd := NewCmdCreateService(testClient)
		silenceCobra(createCmd)
		testCommand(t, createCmd, "", "", "", "", "", []string{"tcp-go-echo:9090"}...)
	}

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdBind(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestUnbindWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "unbind-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Unbind a target from a service",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "unbind-test2",
			args:            []string{"tcp-go-echo-a", "deployment", "tcp-go-echo-a"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "Could not find entry for service interface tcp-go-echo-a",
			realCluster:     true,
		},
		{
			doc:             "unbind-test3",
			args:            []string{"tcp-go-echo", "deployment", "tcp-go-echo"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-unbind-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)

		// create a target deployment as pre-condition
		deployments := c.KubeClient.AppsV1().Deployments(namespace)
		_, err = deployments.Create(tcpDeployment)
		assert.Assert(t, err)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	if *clusterRun {
		createCmd := NewCmdCreateService(testClient)
		silenceCobra(createCmd)
		testCommand(t, createCmd, "", "", "", "", "", []string{"tcp-go-echo:9090"}...)

		bindCmd := NewCmdBind(testClient)
		silenceCobra(bindCmd)
		testCommand(t, bindCmd, "", "", "", "", "", []string{"tcp-go-echo", "deployment", "tcp-go-echo"}...)
	}

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdUnbind(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestVersionWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "version-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Report the version of the Skupper CLI and services",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "version-test2",
			args:            []string{},
			expectedCapture: "client version",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-version-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdVersion(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestDebugDumpWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "debug-dump-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Collect and store skupper logs, config, etc. to compressed archive file",
			expectedError:   "",
			realCluster:     false,
		},
		{
			doc:             "debug-dump-test2",
			args:            []string{"./tmp/dump.txt"},
			expectedCapture: "Skupper dump details written to compressed archive:  ./tmp/dump.txt",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "debug-dump-test3",
			args:            []string{"./tmp/dump"},
			expectedCapture: "Skupper dump details written to compressed archive:  ./tmp/dump.tar.gz",
			expectedOutput:  "",
			expectedError:   "",
			realCluster:     true,
		},
	}

	namespace = "cmd-debug-dump-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--edge", "--console-ingress=none"}...)

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdDebugDump(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}

func TestNetworkStatusWithCluster(t *testing.T) {
	testcases := []testCase{
		{
			doc:             "network-status-test1",
			args:            []string{"--help"},
			expectedCapture: "",
			expectedOutput:  "Shows information about the current site, and connected sites.",
			expectedError:   "",
			realCluster:     true,
		},
		{
			doc:             "network-status-test2",
			args:            []string{"status"},
			expectedCapture: "",
			expectedOutput:  "",
			expectedError:   "",
			outputRegExp:    "^Sites\\:\\n╰─\\s\\[local\\]\\s[0-9a-f-]*\\s-\\s.*\\n\\s{3}URL:\\s\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\n\\s{3}mode:\\s.*\\n\\s{3}name:\\s.*\\n\\s{3}namespace:\\s.*\\n\\s{3}version:\\s.*\\n\\s{3}╰─\\sServices\\:\\n\\s{6}╰─\\sname:\\s.*\\n\\s{9}address:\\s.*\\:\\s\\d{4}\\n\\s{9}protocol:\\s.*\\n\\s{9}╰─\\sTargets\\:\\n\\s{12}╰─\\sname:\\s.*",
			realCluster:     true,
		},
	}

	namespace = "cmd-debug-network-status-cluster-test-" + strings.ToLower(utils.RandomId(4))
	kubeContext = ""
	kubeConfigPath = ""

	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
	}

	cli = NewClient(namespace, kubeContext, kubeConfigPath)

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	skupperInit(t, []string{"--console-ingress=none"}...)
	skupperExpose(t, []string{"service", "tcp-go-echo", "--port", "9090", "--address", "tcp-go-echo-dup"}...)

	// wait for the service controller to be available
	time.Sleep(15 * time.Second)

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	for _, tc := range testcases {
		if tc.realCluster && !*clusterRun {
			continue
		}
		cmd := NewCmdNetworkStatus(testClient)
		silenceCobra(cmd)
		testCommand(t, cmd, tc.doc, tc.expectedError, tc.expectedCapture, tc.expectedOutput, tc.outputRegExp, tc.args...)
	}
}
