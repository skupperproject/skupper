package kube

import (
	jsonencoding "encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/api/types"
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
	kubeClient.CoreV1().ConfigMaps(NS).Create(existingCm)

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

func TestUpdateSkupperServices(t *testing.T) {

	const NS = "test"

	// document test iteration
	type test struct {
		name               string
		hasSkupperServices bool
		fakeUpdateError    bool
		currentData        *map[string]types.ServiceInterface
		changed            []types.ServiceInterface
		deleted            []string
		expectedErr        error
		expectedData       map[string]types.ServiceInterface
	}

	// Fake existing skupper-services' data
	existingData := []types.ServiceInterface{
		{Address: "existing-service-1", Protocol: "http", Ports: []int{8080}},
		{Address: "existing-service-2", Protocol: "tcp", Ports: []int{5672}},
	}
	existingDataMap := map[string]types.ServiceInterface{}
	for _, def := range existingData {
		existingDataMap[def.Address] = def
	}

	// holds the test table
	testTable := []test{
		// skupper-services does not exist
		{
			name:               "skupper-services-not-found",
			hasSkupperServices: false,
			expectedErr:        fmt.Errorf("Could not retrive service definitions from configmap 'skupper-services', Error: "),
		},
		// data is empty and no change
		{
			name:               "data-empty-no-change",
			hasSkupperServices: true,
			expectedData:       map[string]types.ServiceInterface{},
		},
		// data is populated and no change
		{
			name:               "data-populated-no-change",
			hasSkupperServices: true,
			currentData:        &existingDataMap,
			expectedData:       existingDataMap,
		},
		// data is populated, new service added and existing updated
		{
			name:               "data-populated-changed-services",
			hasSkupperServices: true,
			currentData:        &existingDataMap,
			changed: []types.ServiceInterface{
				{Address: "new-service-1", Protocol: "http", Ports: []int{8080}},
				{Address: "existing-service-2", Protocol: "http", Ports: []int{443}},
			},
			expectedData: map[string]types.ServiceInterface{
				"existing-service-1": {Address: "existing-service-1", Protocol: "http", Ports: []int{8080}},
				"existing-service-2": {Address: "existing-service-2", Protocol: "http", Ports: []int{443}},
				"new-service-1":      {Address: "new-service-1", Protocol: "http", Ports: []int{8080}},
			},
		},
		// data is populated and existing deleted
		{
			name:               "data-populated-deleted-services",
			hasSkupperServices: true,
			currentData:        &existingDataMap,
			deleted:            []string{"existing-service-2"},
			expectedData: map[string]types.ServiceInterface{
				"existing-service-1": {Address: "existing-service-1", Protocol: "http", Ports: []int{8080}},
			},
		},
		// data is populated, new service added, existing updated and deleted
		{
			name:               "data-populated-add-upd-delete-services",
			hasSkupperServices: true,
			currentData:        &existingDataMap,
			changed: []types.ServiceInterface{
				{Address: "new-service-1", Protocol: "http", Ports: []int{8080}},
				{Address: "existing-service-2", Protocol: "http", Ports: []int{443}},
			},
			deleted: []string{"existing-service-1"},
			expectedData: map[string]types.ServiceInterface{
				"existing-service-2": {Address: "existing-service-2", Protocol: "http", Ports: []int{443}},
				"new-service-1":      {Address: "new-service-1", Protocol: "http", Ports: []int{8080}},
			},
		},
		// data is populated and update error
		{
			name:               "data-populated-update-error",
			hasSkupperServices: true,
			fakeUpdateError:    true,
			currentData:        &existingDataMap,
			expectedData:       existingDataMap,
			expectedErr:        fmt.Errorf("Failed to update skupper-services config map: "),
		},
	}

	// iterate through testTable to cover UpdateSkupperServices
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			// Creating the fake client
			kubeClient := fake.NewSimpleClientset()

			// If testing update error
			if test.fakeUpdateError {
				kubeClient.Fake.PrependReactor("update", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("fake update error")
				})
			}

			// If requested to create the skupper-services cm
			if test.hasSkupperServices {
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: types.ServiceInterfaceConfigMap,
					},
				}
				if test.currentData != nil {
					cm.Data = map[string]string{}
					for _, def := range *test.currentData {
						jsonDef, _ := jsonencoding.Marshal(def)
						cm.Data[def.Address] = string(jsonDef)
					}
				}
				kubeClient.CoreV1().ConfigMaps(NS).Create(cm)
			}

			// Validating results
			err := UpdateSkupperServices(test.changed, test.deleted, "", NS, kubeClient)
			assert.Equal(t, test.expectedErr == nil, err == nil)
			if err != nil {
				assert.ErrorContains(t, err, test.expectedErr.Error())
			}

			// Validating data
			if test.hasSkupperServices {
				cm, _ := kubeClient.CoreV1().ConfigMaps(NS).Get(types.ServiceInterfaceConfigMap, metav1.GetOptions{})
				assert.Equal(t, test.expectedData == nil, cm.Data == nil)

				// Stringify expectedData first
				if test.expectedData != nil {
					expectedData := map[string]string{}
					for addr, def := range test.expectedData {
						jsonDef, _ := jsonencoding.Marshal(def)
						expectedData[addr] = string(jsonDef)
					}
					assert.Assert(t, reflect.DeepEqual(expectedData, cm.Data),
						fmt.Sprintf("expected data = %v - current data = %v", expectedData, cm.Data))
				}
			}
		})
	}

	/*
		skupper-services cm does not exist error
		data is empty
		data exists
		changed only
		deleted only
		both changed/deleted
		update error
	*/
}
