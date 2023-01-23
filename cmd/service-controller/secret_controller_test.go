package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/skupperproject/skupper/pkg/event"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type MySecretEvent struct {
	Name   string
	Secret *corev1.Secret
}

type MySecretHandler struct {
	Events chan MySecretEvent
}

func newSecretHandler() *MySecretHandler {
	c := make(chan MySecretEvent)
	return &MySecretHandler{
		Events: c,
	}
}

func (h *MySecretHandler) Handle(name string, secret *corev1.Secret) error {
	h.Events <- MySecretEvent{name, secret}
	return nil
}

func (h *MySecretHandler) Wait() *MySecretEvent {
	event := <-h.Events
	return &event
}

func newTestSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Data: map[string][]byte{
			"a": []byte("1"),
			"b": []byte("2"),
		},
	}
}

func TestSecretController(t *testing.T) {
	event.StartDefaultEventStore(nil)
	handler := newSecretHandler()

	namespace := "secret-controller-test"
	kube := fake.NewSimpleClientset()
	controller := NewSecretController("test", "foo=bar", kube, namespace, handler)
	stop := make(chan struct{})
	controller.start(stop)

	name := "my-secret"
	secret := newTestSecret(name)
	_, err := kube.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	assert.Check(t, err, namespace)
	event := handler.Wait()
	assert.Assert(t, event.Secret != nil, namespace)
	assert.Equal(t, event.Name, namespace+"/"+name, namespace)
	assert.Equal(t, event.Secret.ObjectMeta.Name, name, namespace)
	assert.Assert(t, bytes.Equal(event.Secret.Data["a"], []byte("1")), namespace)
	assert.Assert(t, bytes.Equal(event.Secret.Data["b"], []byte("2")), namespace)

	secret.Data["a"] = []byte("A")
	secret.Data["c"] = []byte("3")
	secret.ResourceVersion = "2"
	_, err = kube.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	assert.Check(t, err, namespace)
	event = handler.Wait()
	assert.Equal(t, event.Secret.ObjectMeta.Name, name, namespace)
	assert.Assert(t, bytes.Equal(event.Secret.Data["a"], []byte("A")), namespace)
	assert.Assert(t, bytes.Equal(event.Secret.Data["c"], []byte("3")), namespace)

	err = kube.CoreV1().Secrets(namespace).Delete(context.TODO(), secret.ObjectMeta.Name, metav1.DeleteOptions{})
	event = handler.Wait()
	assert.Check(t, err, namespace)
	assert.Assert(t, event.Secret == nil, namespace)
	assert.Equal(t, event.Name, namespace+"/"+name, namespace)

	controller.stop()
	close(stop)
}
