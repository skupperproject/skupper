package grants

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	fakev2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1/fake"
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
				cert, err := client.GetSkupperClient().SkupperV2alpha1().Certificates(tt.namespace).Get(context.Background(), "skupper-grant-server-ca", metav1.GetOptions{})
				if err != nil {
					t.Error(err)
				}
				assert.Equal(t, cert.Spec.Ca, "")
				assert.Equal(t, cert.Spec.Subject, "SkupperGrantServerCA")
				assert.Equal(t, cert.Spec.Signing, true)
				assert.Equal(t, cert.Spec.Client, false)
				assert.Equal(t, cert.Spec.Server, false)
				assert.DeepEqual(t, cert.ObjectMeta.OwnerReferences, tt.expectedOwnerRefs)

				sa, err := client.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(tt.namespace).Get(context.Background(), "skupper-grant-server", metav1.GetOptions{})
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
			controller := watchers.NewEventProcessor("Controller", client)

			config := &GrantConfig{
				Port:     9090,
				Hostname: "my-pod",
			}
			var found *v2alpha1.SecuredAccess
			handler := func(key string, sa *v2alpha1.SecuredAccess) error {
				found = sa
				return nil
			}
			ac, err := newAutoConfigure(handler, controller, "test", config)
			assert.Assert(t, ac != nil)
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
	client.GetSkupperClient().SkupperV2alpha1().(*fakev2alpha1.FakeSkupperV2alpha1).PrependReactor(e.verb, e.resource, func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
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

// staleGetOnce makes the first get of a resource report NotFound even though it
// exists, reproducing two controllers racing between get and create.
func staleGetOnce(client internalclient.Clients, resource string) {
	done := false
	client.GetSkupperClient().SkupperV2alpha1().(*fakev2alpha1.FakeSkupperV2alpha1).PrependReactor("get", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		if done {
			return false, nil, nil
		}
		done = true
		return true, nil, k8serrors.NewNotFound(schema.GroupResource{Group: "skupper.io", Resource: resource}, action.(k8stesting.GetAction).GetName())
	})
}

func Test_configureConcurrentCreate(t *testing.T) {
	var tests = []struct {
		name     string
		resource string
	}{
		{
			name:     "certificate created by another controller",
			resource: "certificates",
		},
		{
			name:     "securedaccess created by another controller",
			resource: "securedaccesses",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skupperObjects := []runtime.Object{
				tf.cert("skupper-grant-server-ca", "test", "SkupperGrantServerCA", "", true, false, false, ref2),
				tf.securedAccess("skupper-grant-server", "test", map[string]string{"foo": "bar"}, 1234, ref2),
			}
			client, err := fake.NewFakeClient("test", []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)}, skupperObjects, "")
			assert.Assert(t, err)
			staleGetOnce(client, tt.resource)
			ac := &AutoConfigure{
				podname:              "my-pod",
				port:                 1234,
				tlsCredentialsSecret: "skupper-grant-server",
			}
			assert.Assert(t, ac.configure(client, "test"))

			cert, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), "skupper-grant-server-ca", metav1.GetOptions{})
			assert.Assert(t, err)
			assert.DeepEqual(t, cert.ObjectMeta.OwnerReferences, ref1)
			sa, err := client.GetSkupperClient().SkupperV2alpha1().SecuredAccesses("test").Get(context.Background(), "skupper-grant-server", metav1.GetOptions{})
			assert.Assert(t, err)
			assert.DeepEqual(t, sa.ObjectMeta.OwnerReferences, ref1)
		})
	}
}

func Test_configureIsRepeatable(t *testing.T) {
	client, err := fake.NewFakeClient("test", []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)}, nil, "")
	assert.Assert(t, err)
	ac := &AutoConfigure{
		podname:              "my-pod",
		port:                 1234,
		tlsCredentialsSecret: "skupper-grant-server",
	}
	assert.Assert(t, ac.configure(client, "test"))
	assert.Assert(t, ac.configure(client, "test"))
	assert.DeepEqual(t, ac.ownerRefs, ref1)
}

func Test_retryConfigure(t *testing.T) {
	var tests = []struct {
		name     string
		verb     string
		resource string
	}{
		{
			name:     "transient certificate create error",
			verb:     "create",
			resource: "certificates",
		},
		{
			name:     "transient securedaccess create error",
			verb:     "create",
			resource: "securedaccesses",
		},
		{
			name:     "transient certificate update conflict",
			verb:     "update",
			resource: "certificates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var skupperObjects []runtime.Object
			if tt.verb == "update" {
				skupperObjects = []runtime.Object{tf.cert("skupper-grant-server-ca", "test", "SkupperGrantServerCA", "", true, false, false, ref2)}
			}
			client, err := fake.NewFakeClient("test", []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1)}, skupperObjects, "")
			assert.Assert(t, err)
			failures := 2
			client.GetSkupperClient().SkupperV2alpha1().(*fakev2alpha1.FakeSkupperV2alpha1).PrependReactor(tt.verb, tt.resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
				if failures == 0 {
					return false, nil, nil
				}
				failures--
				return true, nil, errors.New("transient error")
			})
			ac := &AutoConfigure{
				podname:              "my-pod",
				port:                 1234,
				tlsCredentialsSecret: "skupper-grant-server",
			}
			assert.ErrorContains(t, ac.configure(client, "test"), "transient error")

			done := make(chan struct{})
			go func() {
				ac.retryConfigure(client, "test", time.Millisecond, 5*time.Millisecond, slog.Default())
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("retryConfigure did not succeed")
			}
			cert, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), "skupper-grant-server-ca", metav1.GetOptions{})
			assert.Assert(t, err)
			assert.DeepEqual(t, cert.ObjectMeta.OwnerReferences, ref1)
			_, err = client.GetSkupperClient().SkupperV2alpha1().SecuredAccesses("test").Get(context.Background(), "skupper-grant-server", metav1.GetOptions{})
			assert.Assert(t, err)
		})
	}
}
