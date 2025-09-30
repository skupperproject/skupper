package certificates

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/internal/certs"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type FakeContext struct {
	controlled  map[string]bool
	labels      map[string]string
	annotations map[string]string
}

func TestCertificateManager(t *testing.T) {
	myCaFixture := fixtureCASecret(t, "my-ca", "test")
	testTable := []struct {
		name                 string
		k8sObjects           []runtime.Object
		skupperObjects       []runtime.Object
		context              ControllerContext
		calls                []*Call
		expectedSecrets      []*corev1.Secret
		expectedCertificates []*skupperv2alpha1.Certificate
	}{
		{
			name: "simple recovery",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, nil, nil),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				addCertificateStatus(certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil), "", "", condition(skupperv2alpha1.CONDITION_TYPE_READY, metav1.ConditionTrue, "Ready", "OK")),
			},
		},
		{
			name: "recovery namespace not controlled",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
			context: fakeContext(),
			expectedSecrets: []*corev1.Secret{
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
		},
		{
			name: "recovery namespace is controlled",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
			context: fakeContext().control("test").label("foo", "bar").annotate("x", "y"),
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, map[string]string{"foo": "bar"}, map[string]string{"x": "y"}),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
		},
		{
			name: "update hosts for same owner",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{},
			context:        fakeContext().control("test").label("foo", "bar").annotate("x", "y"),
			calls: []*Call{
				call("foo", "test").ensure("my-ca", "my-subject", []string{"aaa", "bbb"}, false, true).owner("alice", "49b03ad4-d414-42be-bbb5-b32d7d4ca503"),
				call("foo", "test").ensure("my-ca", "my-subject", []string{"bbb", "yyy", "10.0.0.10"}, false, true).owner("alice", "49b03ad4-d414-42be-bbb5-b32d7d4ca503"),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, map[string]string{"foo": "bar"}, map[string]string{"x": "y"}),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"bbb", "yyy", "10.0.0.10"}, false, true, nil, nil),
			},
		},
		{
			name: "no change required",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{},
			context:        fakeContext().control("test"),
			calls: []*Call{
				call("foo", "test").ensure("my-ca", "my-subject", []string{"aaa", "bbb"}, false, true).owner("alice", "524acdef-d414-42be-bbb5-b32d7d4ca503"),
				call("foo", "test").ensure("my-ca", "my-subject", []string{"aaa", "bbb"}, false, true).owner("alice", "524acdef-d414-42be-bbb5-b32d7d4ca503").eventcount(0),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, nil, nil),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
		},
		{
			name: "merge hosts for different owners",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{},
			context:        fakeContext().control("test"),
			calls: []*Call{
				call("foo", "test").ensure("my-ca", "my-subject", []string{"aaa", "bbb"}, false, true).owner("alice", ""),
				call("foo", "test").ensure("my-ca", "my-subject", []string{"xxx", "yyy"}, false, true).owner("bob", ""),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, nil, nil),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb", "xxx", "yyy"}, false, true, nil, nil),
			},
		},
		{
			name: "update subject",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{},
			context:        fakeContext().control("test").label("foo", "bar").annotate("x", "y"),
			calls: []*Call{
				call("foo", "test").ensure("my-ca", "my-subject", []string{"aaa", "bbb"}, false, true),
				call("foo", "test").ensure("my-ca", "alternative-subject", []string{"aaa", "bbb"}, false, true),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, nil, nil),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "alternative-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
		},
		{
			name:           "ensure ca and certificate",
			k8sObjects:     []runtime.Object{},
			skupperObjects: []runtime.Object{},
			context:        fakeContext().control("test"),
			calls: []*Call{
				call("my-ca", "test").ensureCa("my-cas-subject"),
				call("foo", "test").ensure("my-ca", "my-subject", []string{"xxx", "yyy"}, false, true),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, nil, nil),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				caCertificate("my-ca", "test", "my-cas-subject", map[string]string{"foo": "bar"}, map[string]string{"x": "y"}),
				certificate("foo", "test", "my-ca", "my-subject", []string{"xxx", "yyy"}, false, true, map[string]string{"foo": "bar"}, map[string]string{"x": "y"}),
			},
		},
		{
			name: "update hosts for recovered certificate",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{
				managedWithOwnerHosts(t,
					certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
					metav1.OwnerReference{UID: "aaaa-1a1a1a1a"},
					"aaa",
					"bbb",
				),
			},
			context: fakeContext().control("test").label("foo", "bar").annotate("x", "y"),
			calls: []*Call{
				call("foo", "test").ensure("my-ca", "my-subject", []string{"xxx", "yyy"}, false, true).owner("mallory", "ffff-0f0f0f"),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, map[string]string{"foo": "bar"}, map[string]string{"x": "y"}),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb", "xxx", "yyy"}, false, true, nil, nil),
			},
		},
		{
			name: "prune hosts from deleted owners",
			k8sObjects: []runtime.Object{
				myCaFixture,
				secret("foo", "test", nil, map[string]string{"foo": "bar"}, map[string]string{"x": "y", "internal.skupper.io/controlled": "true"}),
			},
			skupperObjects: []runtime.Object{
				managedWithOwnerHosts(t,
					managedWithOwnerHosts(t,
						certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb", "ccc"}, false, true, nil, nil),
						metav1.OwnerReference{UID: "aaaa-1a1a1a1a"},
						"aaa", "bbb",
					),
					metav1.OwnerReference{UID: "bbbb-bbbb2222"},
					"bbb", "ccc",
				),
			},
			context: fakeContext().control("test").label("foo", "bar").annotate("x", "y"),
			calls: []*Call{
				call("foo", "test").updateCertificate(
					managedWithOwnerHosts(t,
						certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb", "ccc"}, false, true, nil, map[string]string{
							"internal.skupper.io/hosts-bbbb-bbbb2222": "bbb,ccc",
						}),
						metav1.OwnerReference{UID: "aaaa-1a1a1a1a"}, "aaa", "bbb",
					),
				),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, map[string]string{"foo": "bar"}, map[string]string{"x": "y"}),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
		},
		{
			name: "attempt to update non-controlled certificate",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			skupperObjects: []runtime.Object{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
			context: fakeContext().control("test").label("foo", "bar").annotate("x", "y"),
			calls: []*Call{
				call("foo", "test").ensure("my-ca", "my-subject", []string{"xxx", "yyy"}, false, true).mustError(),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, map[string]string{"foo": "bar"}, map[string]string{"x": "y"}),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
		},
		{
			name: "attempted update of non-controlled secret",
			k8sObjects: []runtime.Object{
				myCaFixture,
			},
			calls: []*Call{
				call("my-ca", "test").ensureCa("my-subject"),
			},
			expectedSecrets: []*corev1.Secret{
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				caCertificate("my-ca", "test", "my-subject", nil, nil),
			},
		},
		{
			name: "update of bad secret",
			k8sObjects: []runtime.Object{
				secret("my-ca", "test", map[string][]byte{"tls.crt": []byte("baddata")}, nil, map[string]string{"internal.skupper.io/controlled": "true"}),
			},
			calls: []*Call{
				call("my-ca", "test").ensureCa("my-subject"),
			},
			expectedSecrets: []*corev1.Secret{
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				caCertificate("my-ca", "test", "my-subject", nil, nil),
			},
		},
		{
			name: "certificate deletion",
			k8sObjects: []runtime.Object{
				secret("my-ca", "test", map[string][]byte{"tls.crt": []byte("baddata")}, nil, map[string]string{"internal.skupper.io/controlled": "true"}),
			},
			skupperObjects: []runtime.Object{
				caCertificate("my-ca", "test", "my-subject", nil, nil),
			},
			calls: []*Call{
				call("my-ca", "test").deleteCertificate(),
			},
			expectedSecrets:      []*corev1.Secret{},
			expectedCertificates: []*skupperv2alpha1.Certificate{},
		},
		{
			name: "owned secret missing annotations",
			k8sObjects: []runtime.Object{
				myCaFixture,
				secretWithOwnerRef(secret("foo", "test", nil, nil, nil), metav1.OwnerReference{APIVersion: "skupper.io/v2alpha1", Kind: "Certificate", Name: "foo"}),
			},
			skupperObjects: []runtime.Object{
				certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil),
			},
			expectedSecrets: []*corev1.Secret{
				secret("foo", "test", nil, nil, nil),
				secret("my-ca", "test", nil, nil, nil),
			},
			expectedCertificates: []*skupperv2alpha1.Certificate{
				addCertificateStatus(certificate("foo", "test", "my-ca", "my-subject", []string{"aaa", "bbb"}, false, true, nil, nil), "", "", condition(skupperv2alpha1.CONDITION_TYPE_READY, metav1.ConditionTrue, "Ready", "OK")),
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient("test", tt.k8sObjects, tt.skupperObjects, "")
			if err != nil {
				assert.Assert(t, err)
			}
			processor := watchers.NewEventProcessor("Controller", client)
			mgr := NewCertificateManager(processor)
			if tt.context != nil {
				mgr.SetControllerContext(tt.context)
			}
			mgr.Watch(metav1.NamespaceAll)
			stopCh := make(chan struct{})
			processor.StartWatchers(stopCh)
			processor.WaitForCacheSync(stopCh)
			mgr.Recover()

			processor.TestProcessAll()
			for _, c := range tt.calls {
				callErr := c.invoke(mgr)
				if c.expectErr {
					assert.Assert(t, callErr != nil, "expected call to result in error")
				} else {
					assert.Assert(t, callErr, "unexpected call error")
				}
				for i := 0; i < c.events; i++ {
					processor.TestProcess()
					processor.TestProcess()
				}
			}

			for _, desired := range tt.expectedSecrets {
				actual, err := client.GetKubeClient().CoreV1().Secrets(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				for key, value := range desired.Data {
					assert.Assert(t, actual.Data != nil)
					assert.Equal(t, actual.Data[key], value)
				}
				for key, value := range desired.Labels {
					assert.Assert(t, actual.Labels != nil)
					assert.Equal(t, actual.Labels[key], value)
				}
				for key, value := range desired.Annotations {
					assert.Assert(t, actual.Annotations != nil)
					assert.Equal(t, actual.Annotations[key], value)
				}
			}
			secrets, err := client.GetKubeClient().CoreV1().Secrets(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			assert.Assert(t, err)
			assert.Equal(t, len(tt.expectedSecrets), len(secrets.Items), "wrong number of secrets")
			for _, desired := range tt.expectedCertificates {
				actual, err := client.GetSkupperClient().SkupperV2alpha1().Certificates(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
				assert.Assert(t, err)
				for _, host := range desired.Spec.Hosts {
					assert.Assert(t, cmp.Contains(actual.Spec.Hosts, host))
				}
				assert.Equal(t, len(desired.Spec.Hosts), len(actual.Spec.Hosts))
				assert.Equal(t, desired.Spec.Subject, actual.Spec.Subject)
				verifyStatus(t, desired.Status.Status, actual.Status.Status)
			}
			certificates, err := client.GetSkupperClient().SkupperV2alpha1().Certificates(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
			assert.Assert(t, err)
			assert.Equal(t, len(tt.expectedCertificates), len(certificates.Items), "wrong number of certificates")
		})
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

func secretWithOwnerRef(secret *corev1.Secret, ref metav1.OwnerReference) *corev1.Secret {
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ref}
	return secret
}

// managedWithOwnerHosts sets up a Certificiate with skupper controlled and owner hosts annotations
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

func verifyStatus(t *testing.T, expected skupperv2alpha1.Status, actual skupperv2alpha1.Status) {
	assert.Equal(t, expected.StatusType, actual.StatusType, actual.Message)
	assert.Equal(t, expected.Message, actual.Message)
	for _, condition := range expected.Conditions {
		existing := meta.FindStatusCondition(actual.Conditions, condition.Type)
		assert.Assert(t, existing != nil)
		assert.Equal(t, condition.Status, existing.Status)
		assert.Equal(t, condition.Reason, existing.Reason)
		if condition.Message != "" {
			assert.Equal(t, condition.Message, existing.Message)
		}
	}
}

func addCertificateStatus(cert *skupperv2alpha1.Certificate, statusType skupperv2alpha1.StatusType, message string, conditions ...metav1.Condition) *skupperv2alpha1.Certificate {
	cert.Status = skupperv2alpha1.CertificateStatus{
		Status: skupperv2alpha1.Status{
			StatusType: statusType,
			Message:    message,
			Conditions: conditions,
		},
	}
	return cert
}

func condition(conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
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

type Call struct {
	name       string
	namespace  string
	ca         string
	subject    string
	hosts      []string
	client     bool
	server     bool
	signing    bool
	refs       []metav1.OwnerReference
	events     int
	deleteCert bool
	updateCert *skupperv2alpha1.Certificate
	expectErr  bool
}

func call(name string, namespace string) *Call {
	return &Call{
		name:      name,
		namespace: namespace,
		refs:      fixtureRefs,
	}
}

func (c *Call) invoke(mgr *CertificateManagerImpl) error {
	if c.deleteCert {
		return mgr.processor.GetSkupperClient().SkupperV2alpha1().Certificates(c.namespace).Delete(context.Background(), c.name, metav1.DeleteOptions{})
	}
	if c.updateCert != nil {
		_, err := mgr.processor.GetSkupperClient().SkupperV2alpha1().Certificates(c.namespace).Update(context.Background(), c.updateCert, metav1.UpdateOptions{})
		return err
	}
	if c.signing {
		return mgr.EnsureCA(c.namespace, c.name, c.subject, c.refs)
	}
	return mgr.Ensure(c.namespace, c.name, c.ca, c.subject, c.hosts, c.client, c.server, c.refs)
}

func (c *Call) ensure(ca string, subject string, hosts []string, client bool, server bool) *Call {
	c.ca = ca
	c.subject = subject
	c.hosts = hosts
	c.client = client
	c.server = server
	c.signing = false
	c.events = 1
	return c
}

func (c *Call) ensureCa(subject string) *Call {
	c.subject = subject
	c.signing = true
	c.events = 1
	return c
}

func (c *Call) deleteCertificate() *Call {
	c.deleteCert = true
	c.events = 1
	return c
}

func (c *Call) updateCertificate(updated *skupperv2alpha1.Certificate) *Call {
	c.updateCert = updated
	c.events = 1
	return c
}

func (c *Call) owner(name string, uid string) *Call {
	if uid == "" {
		uid = uuid.NewString()
	}
	c.refs = append(c.refs, metav1.OwnerReference{
		Name: name,
		UID:  types.UID(uid),
	})
	return c
}

func (c *Call) eventcount(events int) *Call {
	c.events = events
	return c
}

func (c *Call) mustError() *Call {
	c.expectErr = true
	return c
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

var (
	fixtureRefs []metav1.OwnerReference = []metav1.OwnerReference{
		{
			Name: "fixture-owner",
			UID:  "0000-0000000",
		},
	}
)
