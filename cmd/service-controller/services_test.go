package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/gorilla/mux"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
)

func encodeServiceOptions(options *ServiceOptions) *bytes.Buffer {
	data, _ := json.Marshal(options)
	return bytes.NewBuffer(data)
}

func createPod(cli kubernetes.Interface, name string, namespace string, labels map[string]string, spec *corev1.PodSpec) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core/v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: *spec,
	}
	created, err := cli.CoreV1().Pods(namespace).Create(pod)
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

func createDeployment(cli kubernetes.Interface, name string, namespace string, image string, ports []corev1.ContainerPort) (*appsv1.Deployment, error) {
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: image,
							Name:  name,
							Ports: ports,
						},
					},
				},
			},
		},
	}
	created, err := cli.AppsV1().Deployments(namespace).Create(dep)
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

func createStatefulSet(cli kubernetes.Interface, name string, namespace string, image string, ports []corev1.ContainerPort) (*appsv1.StatefulSet, error) {
	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: image,
							Name:  name,
							Ports: ports,
						},
					},
				},
			},
		},
	}
	created, err := cli.AppsV1().StatefulSets(namespace).Create(ss)
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

type ResponseChecker interface {
	Check(data []byte) error
}

type ServiceListChecker struct {
	expected []ServiceDefinition
}

func (c *ServiceListChecker) Check(data []byte) error {
	actual := []ServiceDefinition{}
	err := json.Unmarshal(data, &actual)
	if err != nil {
		return err
	}
	expected := map[string]ServiceDefinition{}
	for _, svc := range c.expected {
		expected[svc.Name] = svc
	}
	checked := []string{}
	for _, svc_a := range actual {
		if svc_e, ok := expected[svc_a.Name]; ok {
			err = CheckServiceDefinition(&svc_e, &svc_a)
			if err != nil {
				return err
			}
			checked = append(checked, svc_a.Name)
		} else {
			return fmt.Errorf("Unexpected service %v", svc_a)
		}
	}
	for _, name := range checked {
		delete(expected, name)
	}
	if len(expected) > 0 {
		names := []string{}
		for _, svc := range expected {
			names = append(names, svc.Name)
		}
		return fmt.Errorf("Expected services %v", names)
	}
	return nil
}

type ServiceChecker struct {
	expected ServiceDefinition
}

func (c *ServiceChecker) Check(data []byte) error {
	actual := ServiceDefinition{}
	err := json.Unmarshal(data, &actual)
	if err != nil {
		return err
	}
	return CheckServiceDefinition(&c.expected, &actual)
}

func CheckServiceDefinition(svc_e *ServiceDefinition, svc_a *ServiceDefinition) error {
	if svc_a.Name != svc_e.Name {
		return fmt.Errorf("Expected service name %s, got %s", svc_e.Name, svc_a.Name)
	}
	if svc_a.Protocol != svc_e.Protocol {
		return fmt.Errorf("For %s, expected protocol %s, got %s", svc_e.Name, svc_e.Protocol, svc_a.Protocol)
	}
	if !reflect.DeepEqual(svc_a.Ports, svc_e.Ports) {
		return fmt.Errorf("For %s, expected port %v, got %v", svc_e.Name, svc_e.Ports, svc_a.Ports)
	}
	err := CheckServiceEndpoints(getServiceEndpointsAsMap(svc_a.Endpoints), getServiceEndpointsAsMap(svc_e.Endpoints))
	if err != nil {
		return err
	}
	return nil
}

func getServiceEndpointsAsMap(in []ServiceEndpoint) map[string]ServiceEndpoint {
	out := map[string]ServiceEndpoint{}
	for _, pd := range in {
		out[pd.Name] = pd
	}
	return out
}

func CheckServiceEndpoints(expected map[string]ServiceEndpoint, actual map[string]ServiceEndpoint) error {
	for _, e := range expected {
		if a, ok := actual[e.Name]; ok {
			if !reflect.DeepEqual(a.Ports, e.Ports) {
				return fmt.Errorf("For service endpoint %s, expected port %v, got %v", e.Name, e.Ports, a.Ports)
			}
		} else {
			return fmt.Errorf("Expected service endpoint %s %v", e.Name, e.Ports)
		}
	}
	for _, a := range actual {
		if _, ok := expected[a.Name]; !ok {
			return fmt.Errorf("Unexpected service endpoint %s %v", a.Name, a.Ports)
		}
	}
	return nil
}

func TestServeServices(t *testing.T) {
	event.StartDefaultEventStore(nil)
	var tests = []struct {
		name         string
		method       string
		path         string
		requestBody  io.Reader
		checker      ResponseChecker
		expectedCode int
	}{
		{
			method:       http.MethodGet,
			path:         "/",
			expectedCode: http.StatusNotFound,
		},
		{
			method: http.MethodPost,
			path:   "/services",
			requestBody: encodeServiceOptions(&ServiceOptions{
				Address: "foo",
				Target: ServiceTarget{
					Name: "dep1",
					Type: "deployment",
				},
				Labels: map[string]string{
					"app": "foo",
				},
			}),
			expectedCode: http.StatusOK,
		},
		{
			method: http.MethodPost,
			path:   "/services",
			requestBody: encodeServiceOptions(&ServiceOptions{
				Protocol: "http",
				Ports:    []int{8888},
				Target: ServiceTarget{
					Name: "dep2",
					Type: "deployment",
				},
			}),
			expectedCode: http.StatusOK,
		},
		{
			method: http.MethodPost,
			path:   "/services",
			requestBody: encodeServiceOptions(&ServiceOptions{
				Address:  "dep3",
				Protocol: "http",
				Ports:    []int{8888, 9999},
				Target: ServiceTarget{
					Name: "dep3",
					Type: "deployment",
				},
			}),
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodGet,
			path:         "/services",
			expectedCode: http.StatusOK,
			checker: &ServiceListChecker{
				expected: []ServiceDefinition{
					{
						Name:     "foo",
						Protocol: "tcp",
						Ports:    []int{8181},
						Endpoints: []ServiceEndpoint{
							{
								Name: "pod1",
							},
							{
								Name: "pod2",
							},
						},
					},
					{
						Name:     "dep2",
						Protocol: "http",
						Ports:    []int{8888},
					},
					{
						Name:     "dep3",
						Protocol: "http",
						Ports:    []int{8888, 9999},
					},
				},
			},
		},
		{
			method:       http.MethodGet,
			path:         "/services/foo",
			expectedCode: http.StatusOK,
			checker: &ServiceChecker{
				expected: ServiceDefinition{
					Name:     "foo",
					Protocol: "tcp",
					Ports:    []int{8181},
					Endpoints: []ServiceEndpoint{
						{
							Name: "pod1",
						},
						{
							Name: "pod2",
						},
					},
				},
			},
		},
		{
			method:       http.MethodGet,
			path:         "/services/dep2",
			expectedCode: http.StatusOK,
			checker: &ServiceChecker{
				expected: ServiceDefinition{
					Name:     "dep2",
					Protocol: "http",
					Ports:    []int{8888},
				},
			},
		},
		{
			method:       http.MethodGet,
			path:         "/services/bar",
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodDelete,
			path:         "/services/foo",
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodDelete,
			path:         "/services/bar",
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodDelete,
			path:         "/services",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodGet,
			path:         "/services",
			expectedCode: http.StatusOK,
			checker: &ServiceListChecker{
				expected: []ServiceDefinition{
					{
						Name:     "dep2",
						Protocol: "http",
						Ports:    []int{8888},
					},
					{
						Name:     "dep3",
						Protocol: "http",
						Ports:    []int{8888, 9999},
					},
				},
			},
		},
		{
			method:       http.MethodPut,
			path:         "/services",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "no options supplied",
			method:       http.MethodPost,
			path:         "/services",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "no target name supplied",
			method: http.MethodPost,
			path:   "/services",
			requestBody: encodeServiceOptions(&ServiceOptions{
				Target: ServiceTarget{
					Type: "deployment",
				},
			}),
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "no target type supplied",
			method: http.MethodPost,
			path:   "/services",
			requestBody: encodeServiceOptions(&ServiceOptions{
				Target: ServiceTarget{
					Name: "dep1",
				},
			}),
			expectedCode: http.StatusBadRequest,
		},
	}
	namespace := "services_test"
	cli := &client.VanClient{
		Namespace:  namespace,
		KubeClient: fake.NewSimpleClientset(),
	}
	skupperInitWithController(cli, namespace)
	dep1, err := createDeployment(cli.KubeClient, "dep1", namespace, "nginx", []corev1.ContainerPort{{Name: "myport", ContainerPort: 8181}})
	assert.Check(t, err, namespace)
	_, err = createPod(cli.KubeClient, "pod1", namespace, dep1.Spec.Selector.MatchLabels, &dep1.Spec.Template.Spec)
	_, err = createPod(cli.KubeClient, "pod2", namespace, dep1.Spec.Selector.MatchLabels, &dep1.Spec.Template.Spec)
	assert.Check(t, err, namespace)
	_, err = createDeployment(cli.KubeClient, "dep2", namespace, "nginx", []corev1.ContainerPort{{Name: "myport", ContainerPort: 8181}})
	assert.Check(t, err, namespace)
	_, err = createDeployment(cli.KubeClient, "dep3", namespace, "nginx", []corev1.ContainerPort{{Name: "myport", ContainerPort: 8181}, {Name: "myport2", ContainerPort: 9191}})
	assert.Check(t, err, namespace)
	mgr := newServiceManager(cli)
	router := mux.NewRouter()
	handler := serveServices(mgr)
	router.Handle("/services", handler)
	router.Handle("/services/", handler)
	router.Handle("/services/{name}", handler)
	for _, test := range tests {
		name := test.name
		if name == "" {
			name = test.method + " " + test.path
		}
		req := httptest.NewRequest(test.method, test.path, test.requestBody)
		res := httptest.NewRecorder()

		router.ServeHTTP(res, req)
		responseBody, _ := ioutil.ReadAll(res.Body)
		assert.Equal(t, res.Code, test.expectedCode, name+" "+string(responseBody))
		if test.checker != nil {
			assert.Check(t, test.checker.Check(responseBody), name)
		}
	}
}

type TargetListChecker struct {
	expected []ServiceTarget
}

func (c *TargetListChecker) Check(data []byte) error {
	actual := []ServiceTarget{}
	err := json.Unmarshal(data, &actual)
	if err != nil {
		return err
	}
	expected := map[string]ServiceTarget{}
	for _, tgt := range c.expected {
		expected[tgt.Name] = tgt
	}
	checked := []string{}
	for _, tgt_a := range actual {
		if tgt_e, ok := expected[tgt_a.Name]; ok {
			err = checkServiceTarget(&tgt_e, &tgt_a)
			if err != nil {
				return err
			}
			checked = append(checked, tgt_a.Name)
		} else {
			return fmt.Errorf("Unexpected target %v", tgt_a)
		}
	}
	for _, name := range checked {
		delete(expected, name)
	}
	if len(expected) > 0 {
		names := []string{}
		for _, tgt := range expected {
			names = append(names, tgt.Name)
		}
		return fmt.Errorf("Expected targets %v", names)
	}
	return nil
}

func checkServiceTarget(tgt_e *ServiceTarget, tgt_a *ServiceTarget) error {
	if tgt_a.Name != tgt_e.Name {
		return fmt.Errorf("Expected target name %s, got %s", tgt_e.Name, tgt_a.Name)
	}
	if tgt_a.Type != tgt_e.Type {
		return fmt.Errorf("For %s, expected type %s, got %s", tgt_e.Name, tgt_e.Type, tgt_a.Type)
	}
	err := checkPortDescriptions(getPortDescriptionsAsMap(tgt_e.Ports), getPortDescriptionsAsMap(tgt_a.Ports))
	if err != nil {
		return err
	}
	return nil
}

func checkPortDescriptions(expected map[string]PortDescription, actual map[string]PortDescription) error {
	for _, pde := range expected {
		if pda, ok := actual[pde.Name]; ok {
			if pda.Port != pde.Port {
				if pda.Port != pde.Port {
					return fmt.Errorf("For %s, expected port %d, got %d", pde.Name, pde.Port, pda.Port)
				}
			}
		} else {
			return fmt.Errorf("Expected port description %s %d", pde.Name, pde.Port)
		}
	}
	for _, pda := range actual {
		if _, ok := expected[pda.Name]; !ok {
			return fmt.Errorf("Unexpected port description %s %d", pda.Name, pda.Port)
		}
	}
	return nil
}

func getPortDescriptionsAsMap(in []PortDescription) map[string]PortDescription {
	out := map[string]PortDescription{}
	for _, pd := range in {
		out[pd.Name] = pd
	}
	return out
}

func TestServeServiceTargets(t *testing.T) {
	event.StartDefaultEventStore(nil)
	var tests = []struct {
		name         string
		method       string
		path         string
		requestBody  io.Reader
		checker      ResponseChecker
		expectedCode int
	}{
		{
			method:       http.MethodGet,
			path:         "/targets",
			expectedCode: http.StatusOK,
			checker: &TargetListChecker{
				expected: []ServiceTarget{
					{
						Name: "dep1",
						Type: "deployment",
						Ports: []PortDescription{
							{
								Name: "public",
								Port: 8181,
							},
							{
								Name: "other",
								Port: 9999,
							},
						},
					},
					{
						Name: "dep2",
						Type: "deployment",
						Ports: []PortDescription{
							{
								Name: "http",
								Port: 80,
							},
							{
								Name: "amqp",
								Port: 5672,
							},
						},
					},
					{
						Name: "ss1",
						Type: "statefulset",
						Ports: []PortDescription{
							{
								Name: "https",
								Port: 443,
							},
							{
								Name: "amqps",
								Port: 5671,
							},
						},
					},
				},
			},
		},
		{
			method:       http.MethodDelete,
			path:         "/targets",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodPut,
			path:         "/targets",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodPost,
			path:         "/targets",
			expectedCode: http.StatusMethodNotAllowed,
		},
	}
	namespace := "service_targets_test"
	cli := &client.VanClient{
		Namespace:  namespace,
		KubeClient: fake.NewSimpleClientset(),
	}
	skupperInitWithController(cli, namespace)
	_, err := createDeployment(cli.KubeClient, "dep1", namespace, "nginx", []corev1.ContainerPort{{Name: "public", ContainerPort: 8181}, {Name: "other", ContainerPort: 9999}})
	assert.Check(t, err, namespace)
	_, err = createDeployment(cli.KubeClient, "dep2", namespace, "nginx", []corev1.ContainerPort{{Name: "http", ContainerPort: 80}, {Name: "amqp", ContainerPort: 5672}})
	assert.Check(t, err, namespace)
	_, err = createStatefulSet(cli.KubeClient, "ss1", namespace, "nginx", []corev1.ContainerPort{{Name: "https", ContainerPort: 443}, {Name: "amqps", ContainerPort: 5671}})
	assert.Check(t, err, namespace)
	mgr := newServiceManager(cli)
	router := mux.NewRouter()
	handler := serveTargets(mgr)
	router.Handle("/targets", handler)
	for _, test := range tests {
		name := test.name
		if name == "" {
			name = test.method + " " + test.path
		}
		req := httptest.NewRequest(test.method, test.path, test.requestBody)
		res := httptest.NewRecorder()

		router.ServeHTTP(res, req)
		responseBody, _ := ioutil.ReadAll(res.Body)
		assert.Equal(t, res.Code, test.expectedCode, name+" "+string(responseBody))
		if test.checker != nil {
			assert.Check(t, test.checker.Check(responseBody), name)
		}
	}
}
