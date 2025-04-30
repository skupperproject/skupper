package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorCreate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandConnectorCreateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name: "connector is not created because there is already the same connector in the namespace",
			args: []string{"my-connector", "8080"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Spec: v2alpha1.ConnectorSpec{
						Port:     8080,
						Type:     "tcp",
						Selector: "app=mySelector",
					},
					Status: v2alpha1.ConnectorStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Configured",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectedError: "There is already a connector my-connector created for namespace test",
		},
		{
			name: "connector name and port are not specified",
			args: []string{},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "connector name empty",
			args: []string{"", "8090"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector name must not be empty",
		},
		{
			name: "connector port empty",
			args: []string{"my-name-port-empty", ""},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector port must not be empty",
		},
		{
			name: "connector port not positive",
			args: []string{"my-port-positive", "-45"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector port is not valid: value is not positive",
		},
		{
			name: "connector name and port are not specified",
			args: []string{},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "connector port is not specified",
			args: []string{"my-name"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector name and port must be configured",
		},
		{
			name: "more than two arguments are specified",
			args: []string{"my", "connector", "8080"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "only two arguments are allowed for this command",
		},
		{
			name: "connector name is not valid.",
			args: []string{"my new connector", "8080"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "port is not valid.",
			args: []string{"my-connector-port", "abcd"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "backend",
				Timeout:  1 * time.Minute,
			},
			expectedError: "connector port is not valid: strconv.Atoi: parsing \"abcd\": invalid syntax",
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type", "8080"},
			flags: common.CommandConnectorCreateFlags{
				ConnectorType: "not-valid",
				Timeout:       1 * time.Minute,
				Selector:      "backend",
			},
			expectedError: "connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]",
		},
		{
			name: "routing key is not valid",
			args: []string{"my-connector-rk", "8080"},
			flags: common.CommandConnectorCreateFlags{
				RoutingKey: "not-valid$",
				Timeout:    1 * time.Minute,
				Selector:   "backend",
			},
			expectedError: "routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$",
		},
		{
			name: "tls-secret does not exist",
			args: []string{"my-connector-tls", "8080"},
			flags: common.CommandConnectorCreateFlags{
				TlsCredentials: "not-valid",
				Timeout:        1 * time.Minute,
				Selector:       "backend",
			},
			expectedError: "tls-secret is not valid: does not exist",
		},
		{
			name: "workload is not valid",
			args: []string{"bad-workload", "1234"},
			flags: common.CommandConnectorCreateFlags{
				Workload: "@345",
				Timeout:  1 * time.Minute,
			},
			expectedError: "workload is not valid: workload must include <resource-type>/<resource-name>",
		},
		{
			name: "selector is not valid",
			args: []string{"bad-selector", "1234"},
			flags: common.CommandConnectorCreateFlags{
				Selector: "@#$%",
				Timeout:  20 * time.Second,
			},
			expectedError: "selector is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$",
		},
		{
			name: "timeout is not valid",
			args: []string{"bad-timeout", "8080"},
			flags: common.CommandConnectorCreateFlags{
				Host:    "host",
				Timeout: 0 * time.Second,
			},
			expectedError: "timeout is not valid: duration must not be less than 10s; got 0s",
		},
		{
			name: "host is not valid",
			args: []string{"my-connector-host", "8080"},
			flags: common.CommandConnectorCreateFlags{
				Host:    "not-valid$",
				Timeout: 10 * time.Second,
			},
			expectedError: "host is not valid: a valid IP address or hostname is expected",
		},
		{
			name: "selector/host",
			args: []string{"selector", "1234"},
			flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
				Selector: "app=test",
				Host:     "test",
			},
			expectedError: "If host is configured, cannot configure workload or selector\n" +
				"If selector is configured, cannot configure workload or host",
		},
		{
			name: "workload/host",
			args: []string{"workload", "1234"},
			flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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

			command, err := newCmdConnectorCreateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdConnectorCreate_ValidateWorkload(t *testing.T) {
	type test struct {
		name             string
		args             []string
		flags            common.CommandConnectorCreateFlags
		k8sObjects       []runtime.Object
		skupperObjects   []runtime.Object
		expectedError    string
		expectedSelector string
	}

	testTable := []test{
		{
			name: "workload-no-deployment",
			args: []string{"workload-deployment", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
				Workload: "deployment/backend",
			},
			expectedError: "failed trying to get Deployment specified by workload: deployments.apps \"backend\" not found",
		},
		{
			name: "workload-deployment-no-labels",
			args: []string{"workload-deployment-no-labels", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			args: []string{"workload-deployment", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			expectedError:    "",
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-service",
			args: []string{"workload-no-service", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
				Workload: "service/backend",
			},
			expectedError: "failed trying to get Service specified by workload: services \"backend\" not found",
		},
		{
			name: "workload-service-no-labels",
			args: []string{"workload-service-no-labels", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			args: []string{"workload-service", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			expectedError:    "",
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-daemonset",
			args: []string{"workload-no-daemonset", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
				Workload: "daemonset/backend",
			},
			expectedError: "failed trying to get DaemonSet specified by workload: daemonsets.apps \"backend\" not found",
		},
		{
			name: "workload-daemonset-no-labels",
			args: []string{"workload-daemonset-no-labels", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			args: []string{"workload-daemonset", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			expectedError:    "",
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-statefulset",
			args: []string{"workload-no-statefulset", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
				Workload: "StatefulSet/backend",
			},
			expectedError: "failed trying to get StatefulSet specified by workload: statefulsets.apps \"backend\" not found",
		},
		{
			name: "workload-statefulset-no-labels",
			args: []string{"workload-statefulset-no-labels", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			args: []string{"workload-statefulset", "1234"}, flags: common.CommandConnectorCreateFlags{
				Timeout:  10 * time.Second,
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
			expectedError:    "",
			expectedSelector: "app=backend",
		},
		{
			name:          "wait status is not valid",
			args:          []string{"workload-deployment", "1234"},
			flags:         common.CommandConnectorCreateFlags{Timeout: time.Minute, Wait: "created"},
			expectedError: "status is not valid: value created not allowed. It should be one of this options: [ready configured none]",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorCreateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

			//validate selector is correct
			assert.Check(t, command.selector == test.expectedSelector)
		})
	}
}

func TestCmdConnectorCreate_InputToOptions(t *testing.T) {

	type test struct {
		name                   string
		flags                  common.CommandConnectorCreateFlags
		Connectorname          string
		expectedTlsCredentials string
		expectedHost           string
		expectedSelector       string
		expectedRoutingKey     string
		expectedConnectorType  string
		expectedTimeout        time.Duration
		expectedStatus         string
	}

	testTable := []test{
		{
			name: "test1",
			flags: common.CommandConnectorCreateFlags{
				RoutingKey:          "backend",
				Host:                "",
				Selector:            "app=backend",
				TlsCredentials:      "secret",
				ConnectorType:       "tcp",
				IncludeNotReadyPods: true,
				Workload:            "",
				Timeout:             20 * time.Second,
				Wait:                "ready",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "",
			expectedRoutingKey:     "backend",
			expectedTimeout:        20 * time.Second,
			expectedConnectorType:  "tcp",
			expectedSelector:       "app=backend",
			expectedStatus:         "ready",
		},
		{
			name: "test2",
			flags: common.CommandConnectorCreateFlags{
				RoutingKey:          "backend",
				Host:                "backend",
				Selector:            "",
				TlsCredentials:      "secret",
				ConnectorType:       "tcp",
				IncludeNotReadyPods: true,
				Workload:            "",
				Timeout:             20 * time.Second,
				Wait:                "configured",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "backend",
			expectedRoutingKey:     "backend",
			expectedTimeout:        20 * time.Second,
			expectedConnectorType:  "tcp",
			expectedSelector:       "",
			expectedStatus:         "configured",
		},
		{
			name: "test3",
			flags: common.CommandConnectorCreateFlags{
				RoutingKey:          "",
				Host:                "",
				Selector:            "",
				TlsCredentials:      "secret",
				ConnectorType:       "tcp",
				IncludeNotReadyPods: false,
				Workload:            "",
				Timeout:             30 * time.Second,
				Wait:                "none",
			},
			expectedTlsCredentials: "secret",
			expectedHost:           "",
			expectedRoutingKey:     "test3",
			expectedTimeout:        30 * time.Second,
			expectedConnectorType:  "tcp",
			expectedSelector:       "app=test3",
			expectedStatus:         "none",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			cmd, err := newCmdConnectorCreateWithMocks("test", nil, nil, "")
			assert.Assert(t, err)

			cmd.Flags = &test.flags
			cmd.name = test.name

			cmd.InputToOptions()

			assert.Check(t, cmd.routingKey == test.expectedRoutingKey)
			assert.Check(t, cmd.tlsCredentials == test.expectedTlsCredentials)
			assert.Check(t, cmd.host == test.expectedHost)
			assert.Check(t, cmd.timeout == test.expectedTimeout)
			assert.Check(t, cmd.selector == test.expectedSelector)
			assert.Check(t, cmd.connectorType == test.expectedConnectorType)
			assert.Check(t, cmd.status == test.expectedStatus)
		})
	}
}

func TestCmdConnectorCreate_Run(t *testing.T) {
	type test struct {
		name                string
		connectorName       string
		connectorPort       int
		flags               common.CommandConnectorCreateFlags
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:          "runs ok",
			connectorName: "my-connector-ok",
			connectorPort: 8080,
			flags: common.CommandConnectorCreateFlags{
				ConnectorType:       "tcp",
				RoutingKey:          "keyname",
				TlsCredentials:      "secretname",
				IncludeNotReadyPods: true,
				Selector:            "app=backend",
				Timeout:             10 * time.Second,
			},
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorCreateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			cmd.Flags = &common.CommandConnectorCreateFlags{}
			cmd.name = test.connectorName
			cmd.port = test.connectorPort
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

func TestCmdConnectorCreate_WaitUntil(t *testing.T) {
	type test struct {
		name                string
		status              string
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		expectError         bool
	}

	testTable := []test{
		{
			name: "connector is not ready",
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{},
				},
			},
			expectError: true,
		},
		{
			name:        "connector is not returned",
			expectError: true,
		},
		{
			name:   "connector is ready",
			status: "ready",
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Type:   "Ready",
									Status: "True",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:       "connector is not ready yet, but user waits for configured",
			status:     "configured",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:       "user does not wait",
			status:     "none",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Message:            "OK",
									ObservedGeneration: 1,
									Reason:             "OK",
									Status:             "True",
									Type:               "Configured",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:       "user waits for configured, but site had some errors while being configured",
			status:     "configured",
			k8sObjects: nil,
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector",
						Namespace: "test",
					},
					Status: v2alpha1.ConnectorStatus{
						Status: v2alpha1.Status{
							Conditions: []v1.Condition{
								{
									Message:            "Error",
									ObservedGeneration: 1,
									Reason:             "Error",
									Status:             "False",
									Type:               "Configured",
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorCreateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-connector"
		cmd.timeout = 1 * time.Second
		cmd.namespace = "test"
		cmd.status = test.status

		t.Run(test.name, func(t *testing.T) {

			err := cmd.WaitUntil()
			if test.expectError {
				assert.Check(t, err != nil)
			} else {
				assert.Assert(t, err)
			}
		})
	}
}

// --- helper methods

func newCmdConnectorCreateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdConnectorCreate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdConnectorCreate := &CmdConnectorCreate{
		client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdConnectorCreate, nil
}
