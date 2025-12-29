package watchers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestNamespaceWatcher(t *testing.T) {
	type get struct {
		key       string
		namespace *corev1.Namespace
		err       string
	}
	testTable := []struct {
		name      string
		initial   []runtime.Object
		added     []*corev1.Namespace
		deleted   []*corev1.Namespace
		recovered []*corev1.Namespace
		expected  []*corev1.Namespace
		callbacks []callbackResult[corev1.Namespace]
		gets      []get
		err       string
	}{
		{
			name: "simple",
			initial: []runtime.Object{
				namespace("foo"),
			},
			recovered: []*corev1.Namespace{
				namespace("foo"),
			},
			expected: []*corev1.Namespace{
				namespace("foo"),
			},
			callbacks: []callbackResult[corev1.Namespace]{
				{
					Key: "foo",
					Obj: namespace("foo"),
				},
			},
			gets: []get{
				{
					key:       "foo",
					namespace: namespace("foo"),
				},
				{
					key: "bar",
				},
			},
		},
		{
			name: "deletion",
			initial: []runtime.Object{
				namespace("foo"),
			},
			recovered: []*corev1.Namespace{
				namespace("foo"),
			},
			added: []*corev1.Namespace{
				namespace("bar"),
			},
			deleted: []*corev1.Namespace{
				namespace("foo"),
			},
			expected: []*corev1.Namespace{
				namespace("bar"),
			},
			callbacks: []callbackResult[corev1.Namespace]{
				{
					Key: "foo",
					Obj: namespace("foo"),
				},
				{
					Key: "bar",
					Obj: namespace("bar"),
				},
				{
					Key: "foo",
				},
			},
			gets: []get{
				{
					key: "foo",
				},
				{
					key:       "bar",
					namespace: namespace("bar"),
				},
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient("test", tt.initial, nil, "")
			if err != nil {
				assert.Assert(t, err)
			}
			processor := NewEventProcessor("tester", client)
			handler, getCallbacks := makeHandler[corev1.Namespace](tt.err)
			watcher := processor.WatchNamespaces(nil, handler)
			stopCh := make(chan struct{})
			watcher.Start(stopCh)
			watcher.Sync(stopCh)
			processor.WaitForCacheSync(stopCh)
			recovered := watcher.List()
			for _, expected := range tt.recovered {
				assert.Assert(t, cmp.Contains(recovered, expected))
			}
			assert.Equal(t, len(recovered), len(tt.recovered))

			for i := 0; i < len(tt.initial); i++ {
				processor.TestProcess()
			}
			for _, namespace := range tt.added {
				_, err = client.GetKubeClient().CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
				assert.Assert(t, err)
			}
			for _, namespace := range tt.deleted {
				err = client.GetKubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace.Name, metav1.DeleteOptions{})
				assert.Assert(t, err)
			}
			for i := 0; i < len(tt.added)+len(tt.deleted); i++ {
				processor.TestProcess()
			}
			assert.DeepEqual(t, tt.callbacks, getCallbacks())
			all := watcher.List()
			assert.DeepEqual(t, all, tt.expected)
			for _, get := range tt.gets {
				namespace, err := watcher.Get(get.key)
				if get.err != "" {
					assert.ErrorContains(t, err, tt.err)
				} else {
					assert.Assert(t, err)
					assert.DeepEqual(t, namespace, get.namespace)
				}
			}
		})
	}
}

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestNodeWatcher(t *testing.T) {
	type get struct {
		key  string
		node *corev1.Node
		err  string
	}
	testTable := []struct {
		name      string
		initial   []runtime.Object
		added     []*corev1.Node
		deleted   []*corev1.Node
		recovered []*corev1.Node
		expected  []*corev1.Node
		callbacks []callbackResult[corev1.Node]
		gets      []get
		err       string
	}{
		{
			name: "simple",
			initial: []runtime.Object{
				node("foo"),
			},
			recovered: []*corev1.Node{
				node("foo"),
			},
			expected: []*corev1.Node{
				node("foo"),
			},
			callbacks: []callbackResult[corev1.Node]{
				{
					Key: "foo",
					Obj: node("foo"),
				},
			},
			gets: []get{
				{
					key:  "foo",
					node: node("foo"),
				},
				{
					key: "bar",
				},
			},
		},
		{
			name: "deletion",
			initial: []runtime.Object{
				node("foo"),
			},
			recovered: []*corev1.Node{
				node("foo"),
			},
			added: []*corev1.Node{
				node("bar"),
			},
			deleted: []*corev1.Node{
				node("foo"),
			},
			expected: []*corev1.Node{
				node("bar"),
			},
			callbacks: []callbackResult[corev1.Node]{
				{
					Key: "foo",
					Obj: node("foo"),
				},
				{
					Key: "bar",
					Obj: node("bar"),
				},
				{
					Key: "foo",
				},
			},
			gets: []get{
				{
					key: "foo",
				},
				{
					key:  "bar",
					node: node("bar"),
				},
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient("test", tt.initial, nil, "")
			if err != nil {
				assert.Assert(t, err)
			}
			processor := NewEventProcessor("tester", client)
			handler, getCallbacks := makeHandler[corev1.Node](tt.err)
			watcher := processor.WatchNodes(handler)
			stopCh := make(chan struct{})
			watcher.Start(stopCh)
			watcher.Sync(stopCh)
			processor.WaitForCacheSync(stopCh)
			recovered := watcher.List()
			for _, expected := range tt.recovered {
				assert.Assert(t, cmp.Contains(recovered, expected))
			}
			for i := 0; i < len(tt.initial); i++ {
				processor.TestProcess()
			}
			assert.Equal(t, len(recovered), len(tt.recovered))
			for _, node := range tt.added {
				_, err = client.GetKubeClient().CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
				assert.Assert(t, err)
			}
			for _, node := range tt.deleted {
				err = client.GetKubeClient().CoreV1().Nodes().Delete(context.Background(), node.Name, metav1.DeleteOptions{})
				assert.Assert(t, err)
			}
			for i := 0; i < len(tt.added)+len(tt.deleted); i++ {
				processor.TestProcess()
			}
			assert.DeepEqual(t, tt.callbacks, getCallbacks())
			all := watcher.List()
			assert.DeepEqual(t, all, tt.expected)
			for _, get := range tt.gets {
				node, err := watcher.Get(get.key)
				if get.err != "" {
					assert.ErrorContains(t, err, tt.err)
				} else {
					assert.Assert(t, err)
					assert.DeepEqual(t, node, get.node)
				}
			}
		})
	}
}

func node(name string) *corev1.Node {
	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

type stubErrResourceChangeHandler struct {
	CallCount int
}

func (s *stubErrResourceChangeHandler) Handle(e ResourceChange) error {
	s.CallCount++
	return fmt.Errorf("Handler failed %d", s.CallCount)
}

func (stubErrResourceChangeHandler) Describe(e ResourceChange) string {
	return fmt.Sprintf("StubHandler:%s", e.Key)
}
func (stubErrResourceChangeHandler) Kind() string {
	return "stub"
}

func TestProcessRequeueLimit(t *testing.T) {
	client, _ := fakeclient.NewFakeClient("test", nil, nil, "")
	processor := NewEventProcessor("tester", client)
	processor.queue = workqueue.NewNamedRateLimitingQueue(workqueue.NewItemFastSlowRateLimiter(0, time.Microsecond, 10), "testing")
	stubHandler := stubErrResourceChangeHandler{}
	eventsIn := processor.newEventHandler(&stubHandler)
	eventsIn.AddFunc(node("test"))
	callCount := 0 // set upper bound on how long test will run
	for processor.queue.Len() > 0 && callCount < 1_000 {
		processor.TestProcess()
		callCount++
	}
	assert.Equal(t, stubHandler.CallCount, 6, "Should Requeue 5 times + 1 for the initial event")
}

type callbackResult[T any] struct {
	Key string
	Obj *T
}

func makeHandler[T any](errStr string) (handler func(key string, obj *T) error, getter func() []callbackResult[T]) {
	var (
		actual []callbackResult[T]
		mu     sync.Mutex
	)
	return func(key string, obj *T) error {
			mu.Lock()
			defer mu.Unlock()
			actual = append(actual, callbackResult[T]{
				Key: key,
				Obj: obj,
			})
			if errStr != "" {
				return errors.New(errStr)
			}
			return nil
		}, func() []callbackResult[T] {
			mu.Lock()
			defer mu.Unlock()
			return actual
		}
}
