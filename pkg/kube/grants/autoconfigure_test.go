package grants

import (
	"context"
	"errors"
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	fakev1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1/fake"
	"github.com/skupperproject/skupper/pkg/kube"
)

func Test_configure(t *testing.T) {
	var tests = []struct {
		name              string
		namespace         string
		podname           string
		port              int
		k8sObjects        []runtime.Object
		skupperObjects    []runtime.Object
		prepends          []SkupperClientError
		expectedSelector  map[string]string
		expectedOwnerRefs []metav1.OwnerReference
		expectedError     string
	}{
		{
			name:              "simple",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:              "cert already exists",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			skupperObjects:    []runtime.Object{tf.cert("skupper-grant-server-ca", "test", "SkupperGrantServerCA", "", true, false, false, ref1)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:              "cert has wrong owner refs",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			skupperObjects:    []runtime.Object{tf.cert("skupper-grant-server-ca", "test", "SkupperGrantServerCA", "", true, false, false, ref2)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:              "cert has wrong spec",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			skupperObjects:    []runtime.Object{tf.cert("skupper-grant-server-ca", "test", "ajkfhakjfh", "", false, true, true, ref1)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:              "securedaccess already exists",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			skupperObjects:    []runtime.Object{tf.securedAccess("skupper-grant-server", "test", map[string]string{"foo": "bar"}, 1234, ref1)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:              "securedaccess has wrong refs",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			skupperObjects:    []runtime.Object{tf.securedAccess("skupper-grant-server", "test", map[string]string{"foo": "bar"}, 1234, ref2)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:              "securedaccess has wrong port",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			skupperObjects:    []runtime.Object{tf.securedAccess("skupper-grant-server", "test", map[string]string{"foo": "bar"}, 9090, ref1)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:              "securedaccess has wrong selector",
			podname:           "my-pod",
			port:              1234,
			namespace:         "test",
			k8sObjects:        []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			skupperObjects:    []runtime.Object{tf.securedAccess("skupper-grant-server", "test", map[string]string{"x": "y"}, 1234, ref1)},
			expectedSelector:  map[string]string{"foo": "bar"},
			expectedOwnerRefs: ref1,
		},
		{
			name:          "pod not found",
			podname:       "idontexist",
			namespace:     "other",
			expectedError: "not found",
		},
		{
			name:          "error on cert get",
			podname:       "my-pod",
			port:          1234,
			namespace:     "test",
			k8sObjects:    []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			prepends:      []SkupperClientError{skupperClientError("get", "certificates", "Can't get certificate")},
			expectedError: "Can't get certificate",
		},
		{
			name:          "error on securedaccess get",
			podname:       "my-pod",
			port:          1234,
			namespace:     "test",
			k8sObjects:    []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)},
			prepends:      []SkupperClientError{skupperClientError("get", "securedaccesses", "Can't get securedaccess")},
			expectedError: "Can't get securedaccess",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fake.NewFakeClient("test", tt.k8sObjects, tt.skupperObjects, "")
			if err != nil {
				t.Error(err)
			}
			for _, p := range tt.prepends {
				p.prepend(client)
			}
			ac := &AutoConfigure{
				podname:              tt.podname,
				port:                 tt.port,
				tlsCredentialsSecret: "skupper-grant-server",
			}
			err = ac.configure(client, tt.namespace)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {
				// verify that Certificate & SecuredAccess exist and match expectations
				cert, err := client.GetSkupperClient().SkupperV1alpha1().Certificates(tt.namespace).Get(context.Background(), "skupper-grant-server-ca", metav1.GetOptions{})
				if err != nil {
					t.Error(err)
				}
				assert.Equal(t, cert.Spec.Ca, "")
				assert.Equal(t, cert.Spec.Subject, "SkupperGrantServerCA")
				assert.Equal(t, cert.Spec.Signing, true)
				assert.Equal(t, cert.Spec.Client, false)
				assert.Equal(t, cert.Spec.Server, false)
				assert.DeepEqual(t, cert.ObjectMeta.OwnerReferences, tt.expectedOwnerRefs)

				sa, err := client.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(tt.namespace).Get(context.Background(), "skupper-grant-server", metav1.GetOptions{})
				if err != nil {
					t.Error(err)
				}
				assert.DeepEqual(t, sa.Spec.Selector, tt.expectedSelector)
				assert.DeepEqual(t, sa.Spec.Ports[0].Port, tt.port)
				assert.DeepEqual(t, sa.Spec.Issuer, "skupper-grant-server-ca")
				assert.DeepEqual(t, sa.Spec.Certificate, "skupper-grant-server")
				assert.DeepEqual(t, sa.ObjectMeta.OwnerReferences, tt.expectedOwnerRefs)
			}
		})
	}
}

func Test_newAutoconfigure(t *testing.T) {
	var tests = []struct {
		name          string
		podname       string
		expectedError string
	}{
		{
			name:    "simple",
			podname: "my-pod",
		},
		{
			name:          "failed",
			podname:       "I don't exist",
			expectedError: "not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fake.NewFakeClient("test", []runtime.Object{tf.pod(tt.podname, "test", map[string]string{"foo": "bar"}, ref1)}, nil, "")
			if err != nil {
				t.Error(err)
			}
			controller := kube.NewController("Controller", client)

			config := &GrantConfig{
				Port:     9090,
				Hostname: "my-pod",
			}
			var found *v1alpha1.SecuredAccess
			handler := func(key string, sa *v1alpha1.SecuredAccess) error {
				found = sa
				return nil
			}
			_, err = newAutoConfigure(handler, controller, "test", config)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {
				stopCh := make(chan struct{})
				defer close(stopCh)
				controller.StartWatchers(stopCh)
				assert.Assert(t, controller.WaitForCacheSync(stopCh))
				assert.Assert(t, controller.TestProcess())
				assert.Assert(t, found != nil)
				assert.Equal(t, found.Name, "skupper-grant-server")
				assert.Equal(t, found.Spec.Ports[0].Port, 9090)
			}
		})
	}
}

var ref1 = []metav1.OwnerReference{
	{
		Kind:       "ReplicaSet",
		APIVersion: "v1",
		Name:       "parent1",
		UID:        "0bde3bc8-a4a2-404a-bfbe-44fdf7bf3231",
	},
}
var ref2 = []metav1.OwnerReference{
	{
		Kind:       "ReplicaSet",
		APIVersion: "v1",
		Name:       "parent2",
		UID:        "a40fbe84-f276-4755-bf22-5ba980ab1661",
	},
}

type SkupperClientError struct {
	verb     string
	resource string
	err      string
}

func (e *SkupperClientError) prepend(client internalclient.Clients) {
	client.GetSkupperClient().SkupperV1alpha1().(*fakev1alpha1.FakeSkupperV1alpha1).PrependReactor(e.verb, e.resource, func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New(e.err)
	})
}

func skupperClientError(verb string, resource string, err string) SkupperClientError {
	return SkupperClientError{
		verb:     verb,
		resource: resource,
		err:      err,
	}
}
