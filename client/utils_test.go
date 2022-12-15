package client

import (
	"gotest.tools/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"reflect"
	"testing"
)

func TestContainsAllPolicies(t *testing.T) {
	type test struct {
		name     string
		elements []rbacv1.PolicyRule
		included []rbacv1.PolicyRule
		result   bool
	}

	testTable := []test{
		{
			name: "partially included",
			elements: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps", "pods", "secrets"},
				},
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
			included: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
			result: false,
		},
		{
			name: "partially included with extra rules",
			elements: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps", "pods", "secrets"},
				},
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
			included: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
				{
					Verbs:     []string{"get", "list", "watch"},
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets"},
				},
			},
			result: false,
		},
		{
			name: "all included",
			elements: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps", "pods", "secrets"},
				},
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
			included: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps", "pods", "secrets"},
				},
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
			result: true,
		},
		{
			name: "all included with extra rules",
			elements: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps", "pods", "secrets"},
				},
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
			included: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps", "pods", "secrets"},
				},
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
				{
					Verbs:     []string{"get", "list", "watch"},
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets"},
				},
			},
			result: true,
		},
		{
			name:     "nil rules",
			elements: nil,
			included: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps", "pods", "secrets"},
				},
				{
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
			result: false,
		},
		{
			name:     "all nil",
			elements: nil,
			included: nil,
			result:   false,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			expectedResult := test.result
			actualResult := ContainsAllPolicies(test.elements, test.included)
			assert.Assert(t, reflect.DeepEqual(actualResult, expectedResult))
		})
	}
}
