package kube

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdConnectorUpdate_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandConnectorUpdateFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedErrors []string
	}

	testTable := []test{
		{
			name:           "connector is not updated because get connector returned error",
			args:           []string{"my-connector"},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 10 * time.Second},
			expectedErrors: []string{"connector my-connector must exist in namespace test to be updated"},
		},
		{
			name:           "connector name is not specified",
			args:           []string{},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 10 * time.Second},
			expectedErrors: []string{"connector name must be configured"},
		},
		{
			name:           "connector name is nil",
			args:           []string{""},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 10 * time.Minute},
			expectedErrors: []string{"connector name must not be empty"},
		},
		{
			name:           "more than one argument is specified",
			args:           []string{"my", "connector"},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 10 * time.Minute},
			expectedErrors: []string{"only one argument is allowed for this command"},
		},
		{
			name:           "connector name is not valid.",
			args:           []string{"my new connector"},
			flags:          common.CommandConnectorUpdateFlags{Timeout: 10 * time.Minute},
			expectedErrors: []string{"connector name is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "connector type is not valid",
			args: []string{"my-connector-type"},
			flags: common.CommandConnectorUpdateFlags{
				ConnectorType: "not-valid",
				Timeout:       1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-type",
						Namespace: "test",
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
			expectedErrors: []string{
				"connector type is not valid: value not-valid not allowed. It should be one of this options: [tcp]"},
		},
		{
			name: "routing key is not valid",
			args: []string{"my-connector-rk"},
			flags: common.CommandConnectorUpdateFlags{
				RoutingKey: "not-valid$",
				Timeout:    60 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-rk",
						Namespace: "test",
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
			expectedErrors: []string{
				"routing key is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])*(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])*)*$"},
		},
		{
			name: "tls-secret is not valid",
			args: []string{"my-connector-tls"},
			flags: common.CommandConnectorUpdateFlags{
				TlsCredentials: "test-tls",
				Timeout:        50 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-tls",
						Namespace: "test",
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
			expectedErrors: []string{"tls-secret is not valid: does not exist"},
		},
		{
			name: "port is not valid",
			args: []string{"my-connector-port"},
			flags: common.CommandConnectorUpdateFlags{
				Port:    -1,
				Timeout: 40 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-port",
						Namespace: "test",
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
			expectedErrors: []string{
				"connector port is not valid: value is not positive"},
		},
		{
			name: "workload is not valid",
			args: []string{"bad-workload"},
			flags: common.CommandConnectorUpdateFlags{
				Workload: "!workload",
				Timeout:  70 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-workload",
						Namespace: "test",
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
			expectedErrors: []string{
				"workload is not valid: workload must include <resource-type>/<resource-name>"},
		},
		{
			name: "selector is not valid",
			args: []string{"bad-selector"},
			flags: common.CommandConnectorUpdateFlags{
				Selector: "@#$%",
				Timeout:  1 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-selector",
						Namespace: "test",
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
			expectedErrors: []string{
				"selector is not valid: value does not match this regular expression: ^[A-Za-z0-9=:./-]+$"},
		},
		{
			name: "selector/host",
			args: []string{"selector"},
			flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Selector: "app=test",
				Host:     "test",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "selector",
						Namespace: "test",
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
			expectedErrors: []string{
				"If host is configured, cannot configure workload or selector",
				"If selector is configured, cannot configure workload or host"},
		},
		{
			name: "workload/host",
			args: []string{"workload"},
			flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "deployment/test",
				Host:     "test",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload",
						Namespace: "test",
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
			expectedErrors: []string{
				"If host is configured, cannot configure workload or selector",
				"If workload is configured, cannot configure selector or host"},
		},
		{
			name: "timeout is not valid",
			args: []string{"bad-timeout"},
			flags: common.CommandConnectorUpdateFlags{
				Selector: "selector",
				Timeout:  1 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-timeout",
						Namespace: "test",
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
			expectedErrors: []string{"timeout is not valid: duration must not be less than 10s; got 1s"},
		},
		{
			name: "output is not valid",
			args: []string{"bad-output"},
			flags: common.CommandConnectorUpdateFlags{
				Output:  "not-supported",
				Timeout: 10 * time.Minute,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "bad-output",
						Namespace: "test",
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
			expectedErrors: []string{
				"output type is not valid: value not-supported not allowed. It should be one of this options: [json yaml]"},
		},
		{
			name: "flags all valid",
			args: []string{"my-connector-flags"},
			flags: common.CommandConnectorUpdateFlags{
				RoutingKey:          "routingkeyname",
				TlsCredentials:      "secretname",
				Port:                1234,
				ConnectorType:       "tcp",
				IncludeNotReadyPods: false,
				Timeout:             50 * time.Second,
				Output:              "json",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-flags",
						Namespace: "test",
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
			k8sObjects: []runtime.Object{
				&v12.Secret{
					ObjectMeta: v1.ObjectMeta{
						Name:      "secretname",
						Namespace: "test",
					},
				},
			},
			expectedErrors: []string{},
		},
		{
			name:  "wait status is not valid",
			args:  []string{"backend-connector"},
			flags: common.CommandConnectorUpdateFlags{Timeout: time.Minute, Wait: "created"},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "backend-connector",
						Namespace: "test",
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
			expectedErrors: []string{
				"status is not valid: value created not allowed. It should be one of this options: [ready configured none]",
			},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

		})
	}
}

func TestCmdConnectorUpdate_ValidateWorkload(t *testing.T) {
	type test struct {
		name             string
		args             []string
		flags            common.CommandConnectorUpdateFlags
		k8sObjects       []runtime.Object
		skupperObjects   []runtime.Object
		expectedErrors   []string
		expectedSelector string
		expectedStatus   string
	}

	testTable := []test{
		{
			name: "workload-no-deployment",
			args: []string{"workload-deployment"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "deployment/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-deployment",
						Namespace: "test",
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
			expectedErrors: []string{
				"failed trying to get Deployment specified by workload: deployments.apps \"backend\" not found",
			},
		},
		{
			name: "workload-deployment-no-labels",
			args: []string{"workload-deployment-no-labels"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "deployment/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-deployment-no-labels",
						Namespace: "test",
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
			expectedErrors: []string{"workload, no selector Matchlabels found"},
		},
		{
			name: "workload-deployment",
			args: []string{"workload-deployment"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "deployment/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-deployment",
						Namespace: "test",
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
			expectedErrors:   []string{},
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-service",
			args: []string{"workload-no-service"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "service/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-no-service",
						Namespace: "test",
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
			expectedErrors: []string{
				"failed trying to get Service specified by workload: services \"backend\" not found",
			},
		},
		{
			name: "workload-service-no-labels",
			args: []string{"workload-service-no-labels"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "service/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-service-no-labels",
						Namespace: "test",
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
			expectedErrors: []string{"workload, no selector labels found"},
		},
		{
			name: "workload-service",
			args: []string{"workload-service"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "service/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-service",
						Namespace: "test",
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
			expectedErrors:   []string{},
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-daemonset",
			args: []string{"workload-no-daemonset"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "daemonset/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-no-daemonset",
						Namespace: "test",
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
			expectedErrors: []string{
				"failed trying to get DaemonSet specified by workload: daemonsets.apps \"backend\" not found",
			},
		},
		{
			name: "workload-daemonset-no-labels",
			args: []string{"workload-daemonset-no-labels"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "daemonset/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-daemonset-no-labels",
						Namespace: "test",
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
			expectedErrors: []string{"workload, no selector Matchlabels found"},
		},
		{
			name: "workload-daemonset",
			args: []string{"workload-daemonset"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "DaemonSet/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-daemonset",
						Namespace: "test",
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
			expectedErrors:   []string{},
			expectedSelector: "app=backend",
		},
		{
			name: "workload-no-statefulset",
			args: []string{"workload-no-statefulset"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "StatefulSet/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-no-statefulset",
						Namespace: "test",
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
			expectedErrors: []string{
				"failed trying to get StatefulSet specified by workload: statefulsets.apps \"backend\" not found",
			},
		},
		{
			name: "workload-statefulset-no-labels",
			args: []string{"workload-statefulset-no-labels"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "statefulset/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-statefulset-no-labels",
						Namespace: "test",
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
			expectedErrors: []string{"workload, no selector Matchlabels found"},
		},
		{
			name: "workload-statefulset",
			args: []string{"workload-statefulset"}, flags: common.CommandConnectorUpdateFlags{
				Timeout:  10 * time.Second,
				Output:   "json",
				Workload: "statefulset/backend",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "workload-statefulset",
						Namespace: "test",
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
			expectedErrors:   []string{},
			expectedSelector: "app=backend",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command, err := newCmdConnectorUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, "")
			assert.Assert(t, err)

			command.Flags = &test.flags

			actualErrors := command.ValidateInput(test.args)

			actualErrorsMessages := utils.ErrorsToMessages(actualErrors)

			assert.DeepEqual(t, actualErrorsMessages, test.expectedErrors)

			//validate selector is correct
			assert.Check(t, command.newSettings.selector == test.expectedSelector)
		})
	}
}

func TestCmdConnectorUpdate_Run(t *testing.T) {
	type test struct {
		name                string
		connectorName       string
		flags               ConnectorUpdates
		k8sObjects          []runtime.Object
		skupperObjects      []runtime.Object
		skupperErrorMessage string
		errorMessage        string
	}

	testTable := []test{
		{
			name:          "runs ok",
			connectorName: "my-connector-ok",
			flags: ConnectorUpdates{
				port:                8080,
				connectorType:       "tcp",
				host:                "hostname",
				routingKey:          "keyname",
				tlsCredentials:      "secretname",
				includeNotReadyPods: true,
				workload:            "deployment/backend",
				selector:            "backend",
				timeout:             10 * time.Second,
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-ok",
						Namespace: "test",
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
		},
		{
			name:          "run output json",
			connectorName: "my-connector-json",
			flags: ConnectorUpdates{
				port:                8181,
				connectorType:       "tcp",
				host:                "hostname",
				routingKey:          "keyname",
				tlsCredentials:      "secretname",
				includeNotReadyPods: true,
				workload:            "deployment/backend",
				selector:            "backend",
				timeout:             10 * time.Second,
				output:              "json",
			},
			skupperObjects: []runtime.Object{
				&v2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-connector-json",
						Namespace: "test",
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
		},
	}

	for _, test := range testTable {
		cmd, err := newCmdConnectorUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		t.Run(test.name, func(t *testing.T) {

			cmd.name = test.connectorName
			cmd.newSettings = test.flags
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

func TestCmdConnectorUpdate_WaitUntil(t *testing.T) {
	type test struct {
		name                string
		output              string
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
			name: "connector is ready",
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
			name:   "connector is ready json output",
			output: "json",
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
									Type:   "Configured",
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
									Type:               "Connector",
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
		cmd, err := newCmdConnectorUpdateWithMocks("test", test.k8sObjects, test.skupperObjects, test.skupperErrorMessage)
		assert.Assert(t, err)

		cmd.name = "my-connector"
		cmd.Flags = &common.CommandConnectorUpdateFlags{
			Timeout: 10 * time.Second,
			Output:  test.output,
		}
		cmd.namespace = "test"
		cmd.newSettings.output = cmd.Flags.Output
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

func TestCmdConnectorUpdate_InputToOptions(t *testing.T) {

	type test struct {
		name           string
		args           []string
		flags          common.CommandConnectorUpdateFlags
		expectedStatus string
	}

	testTable := []test{
		{
			name:           "options with waiting status",
			args:           []string{"backend-listener"},
			flags:          common.CommandConnectorUpdateFlags{Wait: "configured"},
			expectedStatus: "configured",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdConnectorUpdate{}
			command.Flags = &test.flags

			command.InputToOptions()

			assert.Check(t, command.status == test.expectedStatus)
		})
	}
}

// --- helper methods

func newCmdConnectorUpdateWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdConnectorUpdate, error) {

	// We make sure the interval is appropriate
	utils.SetRetryProfile(utils.TestRetryProfile)
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	cmdConnectorUpdate := &CmdConnectorUpdate{
		client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		namespace:  namespace,
	}
	return cmdConnectorUpdate, nil
}
