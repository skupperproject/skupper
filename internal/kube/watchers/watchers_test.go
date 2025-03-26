package watchers

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestCallback(t *testing.T) {
	testTable := []struct {
		name string
		err  string
	}{
		{
			name: "simple",
		},
		{
			name: "error",
			err:  "test error",
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient("test", nil, nil, "")
			if err != nil {
				assert.Assert(t, err)
			}
			processor := NewEventProcessor("tester", client)
			stopCh := make(chan struct{})
			processor.StartWatchers(stopCh)
			processor.WaitForCacheSync(stopCh)
			processor.Start(stopCh)
			ch := make(chan string)
			processor.CallbackAfter(0, func(context string) error {
				ch <- context
				if tt.err != "" {
					return errors.New(tt.err)
				}
				return nil
			}, tt.name)
			context := <-ch
			processor.Stop()
			assert.Equal(t, context, tt.name)
		})
	}
}

func TestNamespaceWatcher(t *testing.T) {
	type call struct {
		Key       string
		Namespace *corev1.Namespace
	}
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
		callbacks []call
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
			callbacks: []call{
				{
					Key:       "foo",
					Namespace: namespace("foo"),
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
			callbacks: []call{
				{
					Key:       "foo",
					Namespace: namespace("foo"),
				},
				{
					Key:       "bar",
					Namespace: namespace("bar"),
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
			var actual []call
			handler := func(key string, namespace *corev1.Namespace) error {
				actual = append(actual, call{
					Key:       key,
					Namespace: namespace,
				})
				if tt.err != "" {
					return errors.New(tt.err)
				}
				return nil
			}
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
			for _, namespace := range tt.added {
				_, err = client.GetKubeClient().CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
				assert.Assert(t, err)
			}
			for _, namespace := range tt.deleted {
				err = client.GetKubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace.Name, metav1.DeleteOptions{})
				assert.Assert(t, err)
			}
			for i := 0; i < len(tt.initial)+len(tt.added)+len(tt.deleted); i++ {
				processor.TestProcess()
			}
			assert.DeepEqual(t, tt.callbacks, actual)
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
	type call struct {
		Key  string
		Node *corev1.Node
	}
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
		callbacks []call
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
			callbacks: []call{
				{
					Key:  "foo",
					Node: node("foo"),
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
			callbacks: []call{
				{
					Key:  "foo",
					Node: node("foo"),
				},
				{
					Key:  "bar",
					Node: node("bar"),
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
			var actual []call
			handler := func(key string, node *corev1.Node) error {
				actual = append(actual, call{
					Key:  key,
					Node: node,
				})
				if tt.err != "" {
					return errors.New(tt.err)
				}
				return nil
			}
			watcher := processor.WatchNodes(handler)
			stopCh := make(chan struct{})
			watcher.Start(stopCh)
			watcher.Sync(stopCh)
			processor.WaitForCacheSync(stopCh)
			recovered := watcher.List()
			for _, expected := range tt.recovered {
				assert.Assert(t, cmp.Contains(recovered, expected))
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
			for i := 0; i < len(tt.initial)+len(tt.added)+len(tt.deleted); i++ {
				processor.TestProcess()
			}
			assert.DeepEqual(t, tt.callbacks, actual)
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
