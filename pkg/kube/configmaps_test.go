package kube

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestNewConfigMap(t *testing.T) {

	const NS = "test"
	kubeClient := fake.NewSimpleClientset()

	// Result type to validate
	type result struct {
		cm  *v1.ConfigMap
		err error
	}

	// Test iteration for test table
	type test struct {
		name        string
		cmName      string
		data        *map[string]string
		labels      *map[string]string
		annotations *map[string]string
		owner       *metav1.OwnerReference
		expected    result
	}

	// Add a namespace
	existingCm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-cm",
			Namespace: NS,
			OwnerReferences: []metav1.OwnerReference{
				{Name: "TestNewConfigMap"},
			},
		},
	}
	kubeClient.CoreV1().ConfigMaps(NS).Create(context.TODO(), existingCm, metav1.CreateOptions{})

	// Add a fake reaction when trying to get or create "error-cm"
	kubeClient.Fake.PrependReactor("*", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		switch v := action.(type) {
		case k8stesting.GetAction:
			cmName := v.GetName()
			if cmName == "get-cm-error" {
				return true, nil, fmt.Errorf("fake error getting ConfigMap [%s]", cmName)
			}
		case k8stesting.CreateAction:
			cmName := v.GetObject().(*v1.ConfigMap).Name
			if cmName == "new-cm-error" {
				return true, nil, fmt.Errorf("fake error creating ConfigMap [%s]", cmName)
			}
		}
		return false, nil, nil
	})

	testTable := []test{
		// already exists
		{
			name:        "existing-cm",
			cmName:      "existing-cm",
			data:        &map[string]string{"entry": "value"},
			labels:      &map[string]string{"entry": "value"},
			annotations: &map[string]string{"entry": "value"},
			owner:       &metav1.OwnerReference{Name: "TestNewConfigMap"},
			expected: result{
				cm:  existingCm,
				err: nil,
			},
		},
		// unexpected error retrieving configmap
		{
			name:   "get-cm-error",
			cmName: "get-cm-error",
			expected: result{
				cm:  &v1.ConfigMap{},
				err: fmt.Errorf("Failed to check existing config maps: fake error getting ConfigMap [get-cm-error]"),
			},
		},
		// successful - no data or owner
		{
			name:   "new-cm",
			cmName: "new-cm",
			expected: result{
				cm: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "new-cm",
					},
				},
				err: nil,
			},
		},
		// successful - all data
		{
			name:   "new-cm-all-data",
			cmName: "new-cm-all-data",
			owner: &metav1.OwnerReference{
				Name: "TestNewConfigMap",
			},
			data: &map[string]string{
				"entry1": "value1",
			},
			labels: &map[string]string{
				"entry2": "value2",
			},
			annotations: &map[string]string{
				"entry3": "value3",
			},
			expected: result{
				cm: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "new-cm-all-data",
						Labels: map[string]string{
							"entry2": "value2",
						},
						Annotations: map[string]string{
							"entry3": "value3",
						},
						OwnerReferences: []metav1.OwnerReference{
							{Name: "TestNewConfigMap"},
						},
					},
					Data: map[string]string{
						"entry1": "value1",
					},
				},
				err: nil,
			},
		},
		// error creating
		{
			name:   "new-cm-error",
			cmName: "new-cm-error",
			owner: &metav1.OwnerReference{
				Name: "TestNewConfigMap",
			},
			labels:      &map[string]string{},
			annotations: &map[string]string{},
			data: &map[string]string{
				"entry1": "value1",
			},
			expected: result{
				cm:  nil,
				err: fmt.Errorf("Failed to create config map: fake error creating ConfigMap [new-cm-error]"),
			},
		},
	}

	// Iterate through test table
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			// call NewConfigMap
			cm, err := NewConfigMap(test.cmName, test.data, test.labels, test.annotations, test.owner, NS, kubeClient)
			assert.Equal(t, test.expected.err == nil, err == nil)
			if err != nil {
				assert.Equal(t, test.expected.err.Error(), err.Error())
			}
			assert.Equal(t, test.expected.cm == nil, cm == nil)
			if cm != nil {
				assert.Equal(t, test.expected.cm.ObjectMeta.Name, cm.ObjectMeta.Name)
				assert.Equal(t, test.expected.cm.Data == nil, cm.Data == nil)
				if cm.Data != nil {
					assert.Assert(t, reflect.DeepEqual(test.expected.cm.Data, cm.Data))
				}
				assert.Equal(t, test.expected.cm.OwnerReferences == nil, cm.OwnerReferences == nil)
				if cm.OwnerReferences != nil {
					assert.Assert(t, reflect.DeepEqual(test.expected.cm.OwnerReferences, cm.OwnerReferences))
				}
			}
		})
	}
}
