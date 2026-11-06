package certificates

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/certs"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type FakeContext struct {
	controlled  map[string]bool
	labels      map[string]string
	annotations map[string]string
}

// Authoring changes Certificate resources only. Issuance and Ready are tested
// through Handle in reconcile_test.go; no signing material is needed here.
func TestEnsureAuthoring(t *testing.T) {
	alice := metav1.OwnerReference{Name: "alice", UID: "alice"}
	bob := metav1.OwnerReference{Name: "bob", UID: "bob"}
	type request struct {
		subject string
		hosts   []string
		owner   metav1.OwnerReference
	}
	for _, tc := range []struct {
		name          string
		initial       *skupperv2alpha1.Certificate
		calls         []request
		subject       string
		hosts         []string
		owners        []metav1.OwnerReference
		contributions map[string]string
		expectError   bool
	}{
		{
			name:    "update hosts for same owner",
			calls:   []request{{"server", []string{"aaa", "bbb"}, alice}, {"server", []string{"bbb", "yyy", "10.0.0.10"}, alice}},
			subject: "server", hosts: []string{"10.0.0.10", "bbb", "yyy"}, owners: []metav1.OwnerReference{alice},
			contributions: map[string]string{"alice": "10.0.0.10,bbb,yyy"},
		},
		{
			name:    "merge hosts for different owners",
			calls:   []request{{"server", []string{"aaa", "bbb"}, alice}, {"server", []string{"bbb", "yyy"}, bob}},
			subject: "server", hosts: []string{"aaa", "bbb", "yyy"}, owners: []metav1.OwnerReference{alice, bob},
			contributions: map[string]string{"alice": "aaa,bbb", "bob": "bbb,yyy"},
		},
		{
			name:    "update subject",
			calls:   []request{{"server", []string{"aaa", "bbb"}, alice}, {"replacement", []string{"aaa", "bbb"}, alice}},
			subject: "replacement", hosts: []string{"aaa", "bbb"}, owners: []metav1.OwnerReference{alice},
			contributions: map[string]string{"alice": "aaa,bbb"},
		},
		{
			name:    "add owner to recovered Certificate",
			initial: managedWithOwnerHosts(t, certificate("leaf", "test", "ca", "server", []string{"aaa", "bbb"}, false, true, nil, nil), alice, "aaa", "bbb"),
			calls:   []request{{"server", []string{"xxx", "yyy"}, bob}},
			subject: "server", hosts: []string{"aaa", "bbb", "xxx", "yyy"}, owners: []metav1.OwnerReference{alice, bob},
			contributions: map[string]string{"alice": "aaa,bbb", "bob": "xxx,yyy"},
		},
		{
			name:    "refuse unmanaged Certificate",
			initial: certificate("leaf", "test", "ca", "server", []string{"aaa", "bbb"}, false, true, nil, nil),
			calls:   []request{{"replacement", []string{"xxx"}, alice}}, expectError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var objects []runtime.Object
			if tc.initial != nil {
				objects = append(objects, tc.initial)
			}
			client, err := fakeclient.NewFakeClient("test", nil, objects, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			m.SetControllerContext(fakeContext().control("test").label("foo", "bar").annotate("x", "y"))
			kube := client.GetKubeClient().(*kubefake.Clientset)
			kube.ClearActions()
			for _, r := range tc.calls {
				err := m.Ensure("test", "leaf", "ca", r.subject, r.hosts, false, true, []metav1.OwnerReference{r.owner})
				if tc.expectError {
					assert.Error(t, err, `certificate "leaf" exists but is not controlled by skupper`)
				} else {
					assert.NilError(t, err)
				}
			}
			assert.Equal(t, len(kube.Actions()), 0, "authoring must not access Secrets")
			actual, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), "leaf", metav1.GetOptions{})
			assert.NilError(t, err)
			if tc.expectError {
				assert.DeepEqual(t, actual, tc.initial)
				return
			}
			assert.DeepEqual(t, actual.Spec, skupperv2alpha1.CertificateSpec{Ca: "ca", Subject: tc.subject, Hosts: tc.hosts, Server: true})
			assert.DeepEqual(t, actual.OwnerReferences, tc.owners)
			assert.Equal(t, actual.Labels["foo"], "bar")
			assert.Equal(t, actual.Annotations["x"], "y")
			assert.Equal(t, actual.Annotations["internal.skupper.io/controlled"], "true")
			for uid, hosts := range tc.contributions {
				assert.Equal(t, actual.Annotations["internal.skupper.io/hosts-"+uid], hosts)
			}
			assert.DeepEqual(t, actual.Status, skupperv2alpha1.CertificateStatus{})
		})
	}
}

func TestEnsureCAAndLeaf(t *testing.T) {
	client, err := fakeclient.NewFakeClient("test", nil, nil, "")
	assert.NilError(t, err)
	m := newScopeTest(t, client)
	m.SetControllerContext(fakeContext().control("test").label("foo", "bar").annotate("x", "y"))
	kube := client.GetKubeClient().(*kubefake.Clientset)
	kube.ClearActions()
	refs := []metav1.OwnerReference{{Name: "site", UID: "site"}}
	assert.NilError(t, m.EnsureCA("test", "ca", "issuer", refs))
	assert.NilError(t, m.Ensure("test", "leaf", "ca", "client", []string{"example.test"}, true, false, refs))
	assert.Equal(t, len(kube.Actions()), 0)
	for name, spec := range map[string]skupperv2alpha1.CertificateSpec{
		"ca":   {Subject: "issuer", Signing: true},
		"leaf": {Ca: "ca", Subject: "client", Hosts: []string{"example.test"}, Client: true},
	} {
		actual, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), name, metav1.GetOptions{})
		assert.NilError(t, err)
		assert.DeepEqual(t, actual.Spec, spec)
		assert.DeepEqual(t, actual.OwnerReferences, refs)
		assert.Equal(t, actual.Labels["foo"], "bar")
		assert.Equal(t, actual.Annotations["x"], "y")
		assert.DeepEqual(t, actual.Status, skupperv2alpha1.CertificateStatus{})
	}
}

func secret(name string, namespace string, data map[string][]byte, labels map[string]string, annotations map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: data,
	}
}

// managedWithOwnerHosts sets up a Certificate with skupper controlled and owner hosts annotations
func managedWithOwnerHosts(t *testing.T, cert *skupperv2alpha1.Certificate, ref metav1.OwnerReference, hosts ...string) *skupperv2alpha1.Certificate {
	t.Helper()
	cert.ObjectMeta.OwnerReferences = append(cert.ObjectMeta.OwnerReferences, ref)
	specHosts := make(map[string]struct{}, len(cert.Spec.Hosts))
	for _, specHost := range cert.Spec.Hosts {
		specHosts[specHost] = struct{}{}
	}
	for _, host := range hosts {
		if _, ok := specHosts[host]; ok {
			continue
		}
		cert.Spec.Hosts = append(cert.Spec.Hosts, host)
	}
	setAnnotation(&cert.ObjectMeta, annotationKeySkupperControlled, "")
	setAnnotation(
		&cert.ObjectMeta,
		certificateHostsAnnotationKey(string(ref.UID)),
		strings.Join(hosts, ","),
	)
	return cert
}

func certificate(name string, namespace string, ca string, subject string, hosts []string, client bool, server bool, labels map[string]string, annotations map[string]string) *skupperv2alpha1.Certificate {
	return &skupperv2alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: skupperv2alpha1.CertificateSpec{
			Ca:      ca,
			Subject: subject,
			Hosts:   hosts,
			Client:  client,
			Server:  server,
			Signing: false,
		},
	}
}

func caCertificate(name string, namespace string, subject string, labels map[string]string, annotations map[string]string) *skupperv2alpha1.Certificate {
	return &skupperv2alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: skupperv2alpha1.CertificateSpec{
			Subject: subject,
			Signing: true,
		},
	}
}

func fakeContext() *FakeContext {
	return &FakeContext{
		controlled:  map[string]bool{},
		labels:      map[string]string{},
		annotations: map[string]string{},
	}
}

func (c *FakeContext) control(namespace string) *FakeContext {
	c.controlled[namespace] = true
	return c
}

func (c *FakeContext) label(key, value string) *FakeContext {
	c.labels[key] = value
	return c
}

func (c *FakeContext) annotate(key, value string) *FakeContext {
	c.annotations[key] = value
	return c
}

func (c *FakeContext) IsControlled(namespace string) bool {
	return c.controlled[namespace]
}

func (c *FakeContext) SnapshotMetadata(namespace string) *labels.LabelsAndAnnotations {
	policy := labels.NewLabelsAndAnnotations(namespace)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "test-policy", Labels: map[string]string{"skupper.io/label-template": "true"}, Annotations: c.annotations}}
	for k, v := range c.labels {
		cm.Labels[k] = v
	}
	_ = policy.Update(namespace+"/test-policy", cm)
	return policy.Snapshot(namespace)
}

func (c *FakeContext) SetLabels(namespace string, name string, kind string, labels map[string]string) bool {
	changed := false
	for k, v := range c.labels {
		if labels[k] != v {
			labels[k] = v
			changed = true
		}
	}
	return changed
}

func (c *FakeContext) SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool {
	changed := false
	for k, v := range c.annotations {
		if annotations[k] != v {
			annotations[k] = v
			changed = true
		}
	}
	return changed
}

func fixtureCASecret(t *testing.T, name, namespace string) *corev1.Secret {
	t.Helper()
	secret, err := certs.GenerateSecret(name, "skupper test CA", nil, time.Hour*8, nil)
	if err != nil {
		t.Error(err)
	}
	secret.Namespace = namespace
	return secret
}
