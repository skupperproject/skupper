package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorGenerate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandConnectorGenerateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name: "connector name and port are not specified",
			args: []string{},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "connector name empty",
			args: []string{"", "8090"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name must not be empty",
		},
		{
			name: "connector port empty",
			args: []string{"my-name-port-empty", ""},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector port must not be empty",
		},
		{
			name: "connector port not positive",
			args: []string{"my-port-positive", "-45"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector port is not valid: value is not positive",
		},
		{
			name: "connector name and port are not specified",
			args: []string{},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "connector port is not specified",
			args: []string{"my-name"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "more than two arguments are specified",
			args: []string{"my", "connector", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "only two arguments are allowed for this command",
		},
		{
			name: "connector name is not valid.",
			args: []string{"my new connector", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "port is not valid.",
			args: []string{"my-connector-port", "abcd"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "backend",
			},
			expectedError: "connector port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax",
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				ConnectorType: "not-valid",
				Selector:      "backend",
			},
			expectedError: "connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name: "routing key is not valid",
			args: []string{"my-connector-rk", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				RoutingKey: "not-valid$",
				Selector:   "backend",
			},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "tls-credentials does not exist",
			args: []string{"my-connector-tls", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				TlsCredentials: "not-valid",
				Selector:       "backend",
			},
			expectedError: "tlsCredentials is not valid: does not exist",
		},
		{
			name: "workload is not valid",
			args: []string{"bad-workload", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Workload: "@345",
			},
			expectedError: "workload is not valid: workload must include <resource-type>/<resource-name>",
		},
		{
			name: "selector is not valid",
			args: []string{"bad-selector", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Selector: "@#$%",
			},
			expectedError: "selector is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$",
		},
		{
			name: "host is not valid",
			args: []string{"my-connector-host", "8080"},
			flags: common.CommandConnectorGenerateFlags{
				Host: "not-valid$"},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name: "output is not valid",
			args: []string{"bad-output", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Host:   "host",
				Output: "not-supported",
			},
			expectedError: "output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]",
		},
		{
			name: "selector/host",
			args: []string{"selector", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Selector: "app=test",
				Host:     "test",
			},
			expectedError: "If host is configured, cannot configure workload or selector\n" +
				"If selector is configured, cannot configure workload or host",
		},
		{
			name: "workload/host",
			args: []string{"workload", "1234"},
			flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "deployment/test",
				Host:     "test",
			},
			k8sObjects: []runtime.Object{
				&appsv1.Deployment{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &v1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "backend",
							},
						},
					},
				},
			},
			expectedError: "If host is configured, cannot configure workload or selector\n" +
				"If workload is configured, cannot configure selector or host",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}

func TestCmdConnectorGenerate_ValidateWorkload(t *testing.T) {
	type test struct {
		name             string
		args             []string
		flags            common.CommandConnectorGenerateFlags
		k8sObjects       []runtime.Object
		skupperObjects   []runtime.Object
		expectedError    string
		expectedSelector string
	}

	testTable := []test{
		{
			name: "workload-no-deployment",
			args: []string{"workload-deployment", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "deployment/backend",
			},
			expectedError: "failed trying to get Deployment specified by workload: deployments.apps \"backend\" not found",
		},
		{
			name: "workload-deployment-no-labels",
			args: []string{"workload-deployment-no-labels", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "deployment/backend",
			},
			k8sObjects: []runtime.Object{
				&appsv1.Deployment{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &v1.LabelSelector{},
					},
				},
			},
			expectedError: "workload, no selector Matchlabels found",
		},
		{
			name: "workload-deployment",
			args: []string{"workload-deployment", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "deployment/backend",
			},
			k8sObjects: []runtime.Object{
				&appsv1.Deployment{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &v1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "backend",
							},
						},
					},
				},
			},
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-service",
			args: []string{"workload-no-service", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "service/backend",
			},
			expectedError: "failed trying to get Service specified by workload: services \"backend\" not found",
		},
		{
			name: "workload-service-no-labels",
			args: []string{"workload-service-no-labels", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "service/backend",
			},
			k8sObjects: []runtime.Object{
				&v12.Service{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Service",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: v12.ServiceSpec{},
				},
			},
			expectedError: "workload, no selector labels found",
		},
		{
			name: "workload-service",
			args: []string{"workload-service", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "service/backend",
			},
			k8sObjects: []runtime.Object{
				&v12.Service{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Service",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: v12.ServiceSpec{
						Selector: map[string]string{
							"app": "backend",
						},
					},
				},
			},
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-daemonset",
			args: []string{"workload-no-daemonset", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "daemonset/backend",
			},
			expectedError: "failed trying to get DaemonSet specified by workload: daemonsets.apps \"backend\" not found",
		},
		{
			name: "workload-daemonset-no-labels",
			args: []string{"workload-daemonset-no-labels", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "daemonset/backend",
			},
			k8sObjects: []runtime.Object{
				&appsv1.DaemonSet{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "daemonset",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: appsv1.DaemonSetSpec{
						Selector: &v1.LabelSelector{},
					},
				},
			},
			expectedError: "workload, no selector Matchlabels found",
		},
		{
			name: "workload-daemonset",
			args: []string{"workload-daemonset", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "DaemonSet/backend",
			},
			k8sObjects: []runtime.Object{
				&appsv1.DaemonSet{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "DaemonSet",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: appsv1.DaemonSetSpec{
						Selector: &v1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "backend",
							},
						},
					},
				},
			},
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-statefulset",
			args: []string{"workload-no-statefulset", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "StatefulSet/backend",
			},
			expectedError: "failed trying to get StatefulSet specified by workload: statefulsets.apps \"backend\" not found",
		},
		{
			name: "workload-statefulset-no-labels",
			args: []string{"workload-statefulset-no-labels", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "statefulset/backend",
			},
			k8sObjects: []runtime.Object{
				&appsv1.StatefulSet{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "statefulset",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &v1.LabelSelector{},
					},
				},
			},
			expectedError: "workload, no selector Matchlabels found",
		},
		{
			name: "workload-statefulset",
			args: []string{"workload-statefulset", "1234"}, flags: common.CommandConnectorGenerateFlags{
				Output:   "json",
				Workload: "statefulset/backend",
			},
			k8sObjects: []runtime.Object{
				&appsv1.StatefulSet{
					TypeMeta: v1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
					},
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend",
						Namespace: "test",
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &v1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "backend",
							},
						},
					},
				},
			},
			expectedSelector: "app=backend",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

			//validate selector is correct
			assert.Check(t, command.selector == test.expectedSelector)
		})
	}
}

func TestCmdConnectorGenerate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		flags                  common.CommandConnectorGenerateFlags
		Connectorname          string
		expectedTlsCredentials string
		expectedHost           string
		expectedSelector       string
		expectedRoutingKey     string
		expectedConnectorType  string
		expectedOutput         string
	}

	testTable := []test{
		{
			name:                   "test1",
			flags:                  common.CommandConnectorGenerateFlags{"backend", "", "app=backend", "secret", "tcp", true, "", "json"},
			expectedTlsCredentials: "secret",
			expectedHost:           "",
			expectedRoutingKey:     "backend",
			expectedConnectorType:  "tcp",
			expectedOutput:         "json",
			expectedSelector:       "app=backend",
		},
		{
			name:                   "test2",
			flags:                  common.CommandConnectorGenerateFlags{"backend", "backend", "", "secret", "tcp", true, "", "json"},
			expectedTlsCredentials: "secret",
			expectedHost:           "backend",
			expectedRoutingKey:     "backend",
			expectedConnectorType:  "tcp",
			expectedOutput:         "json",
			expectedSelector:       "",
		},
		{
			name:                   "test3",
			flags:                  common.CommandConnectorGenerateFlags{"", "", "", "secret", "tcp", false, "", "yaml"},
			expectedTlsCredentials: "secret",
			expectedHost:           "",
			expectedRoutingKey:     "test3",
			expectedConnectorType:  "tcp",
			expectedOutput:         "yaml",
			expectedSelector:       "app=test3",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdConnectorGenerateWithMocks("test", nil, nil, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags
			cmd.name = test.name

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.output == test.expectedOutput)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.selector == test.expectedSelector)
			assert.Check(t, cmd.connectorType == test.expectedConnectorType)
		})
	}
}

func TestCmdConnectorGenerate_Run(t *testing.T) {
	type test struct {
		name                string
		connectorName       string
		connectorPort       int
		flags               common.CommandConnectorGenerateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:          "runs ok yaml",
			connectorName: "my-connector-ok",
			connectorPort: 8080,
			flags: common.CommandConnectorGenerateFlags{
				ConnectorType:       "tcp",
				RoutingKey:          "keyname",
				TlsCredentials:      "secretname",
				IncludeNotReadyPods: true,
				Selector:            "app=backend",
				Output:              "yaml",
			},
		},
		{
			name:          "run ok json",
			connectorName: "my-connector-json",
			connectorPort: 8080,
			flags: common.CommandConnectorGenerateFlags{
				ConnectorType:       "tcp",
				Host:                "hostname",
				RoutingKey:          "keyname",
				TlsCredentials:      "secretname",
				IncludeNotReadyPods: true,
				Output:              "json",
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorGenerateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			cmd.Flags = &common.CommandConnectorGenerateFlags{}
			cmd.name = test.connectorName
			cmd.port = test.connectorPort
			cmd.output = test.flags.Output
			cmd.namespace = "test"

			err := cmd.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods

func newCmdConnectorGenerateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdConnectorGenerate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdConnectorGenerate := &CmdConnectorGenerate{
		client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdConnectorGenerate, nil
}
