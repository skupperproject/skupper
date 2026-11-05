package certificates

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/certs"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperfake "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

// API state and cached observations are deliberately independent. No informer
// runs in scope tests; observe is the only way API writes enter these stores.
type scopeTest struct {
	*CertificateManagerImpl
	t                     *testing.T
	certificates, secrets cache.Store
}

func newScopeTest(t *testing.T, client internalclient.Clients) *scopeTest {
	t.Helper()
	p := watchers.NewEventProcessor("scope-test", client)
	t.Cleanup(p.Stop)
	m := NewCertificateManager(p)
	ci := cache.NewSharedIndexInformer(&cache.ListWatch{}, &skupperv2alpha1.Certificate{}, 0, cache.Indexers{})
	si := cache.NewSharedIndexInformer(&cache.ListWatch{}, &corev1.Secret{}, 0, cache.Indexers{})
	m.certificateWatcher = watchers.NewResourceWatcher(m.certificateChanged, skupperv2alpha1.SchemeGroupVersion, ci)
	m.secretWatcher = watchers.NewResourceWatcher(m.secretChanged, corev1.SchemeGroupVersion, si)
	h := &scopeTest{m, t, ci.GetStore(), si.GetStore()}
	cs, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("").List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	for i := range cs.Items {
		assert.NilError(t, h.certificates.Add(cs.Items[i].DeepCopy()))
	}
	ss, err := client.GetKubeClient().CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	for i := range ss.Items {
		assert.NilError(t, h.secrets.Add(ss.Items[i].DeepCopy()))
	}
	return h
}

func (h *scopeTest) observe(key string) {
	h.t.Helper()
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	assert.NilError(h.t, err)
	c, err := h.processor.GetSkupperClient().SkupperV2alpha1().Certificates(ns).Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		assert.NilError(h.t, h.certificates.Delete(&skupperv2alpha1.Certificate{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}))
	} else {
		assert.NilError(h.t, err)
		assert.NilError(h.t, h.certificates.Update(c.DeepCopy()))
	}
	s, err := h.processor.GetKubeClient().CoreV1().Secrets(ns).Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		assert.NilError(h.t, h.secrets.Delete(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}))
	} else {
		assert.NilError(h.t, err)
		assert.NilError(h.t, h.secrets.Update(s.DeepCopy()))
	}
}

type metadataContext struct {
	*FakeContext
	policy *labels.LabelsAndAnnotations
}

func (c metadataContext) SnapshotMetadata(namespace string) *labels.LabelsAndAnnotations {
	return c.policy.Snapshot(namespace)
}

func TestMetadataDerivation(t *testing.T) {
	ca := fixtureCASecret(t, "ca", "test")
	old, err := certs.GenerateSecret("leaf", "server", []string{"old.example"}, time.Hour, ca)
	assert.NilError(t, err)
	old.Namespace = "test"
	old.Annotations = map[string]string{"internal.skupper.io/controlled": "true", "internal.skupper.io/hosts": "old.example"}
	for _, existing := range []*corev1.Secret{nil, old} {
		name := "create"
		if existing != nil {
			name = "regenerate"
		}
		t.Run(name, func(t *testing.T) {
			definition := certificate("leaf", "test", "ca", "server", []string{"new.example"}, false, true, nil, nil)
			objects := []runtime.Object{ca}
			if existing != nil {
				objects = append(objects, existing)
			}
			client, err := fakeclient.NewFakeClient("test", objects, []runtime.Object{definition}, "")
			assert.NilError(t, err)
			policy := labels.NewLabelsAndAnnotations("test")
			template := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "policy",
				Labels: map[string]string{"skupper.io/label-template": "true"}, Annotations: map[string]string{"selected": "captured"}},
				Data: map[string]string{"kind": "Secret", "labelSelector": "internal.skupper.io/hosts=new.example"}}
			assert.NilError(t, policy.Update("test/policy", template))
			mgr := newScopeTest(t, client)
			mgr.SetControllerContext(metadataContext{fakeContext().control("test"), policy})
			assert.NilError(t, mgr.Handle(watchers.ResourceChange{Key: "test/leaf"}))
			actual, err := client.GetKubeClient().CoreV1().Secrets("test").Get(context.Background(), "leaf", metav1.GetOptions{})
			assert.NilError(t, err)
			assert.Equal(t, actual.Annotations["selected"], "captured", "selector must see derived hosts")
			assert.Equal(t, actual.Annotations["internal.skupper.io/hosts"], "new.example")
			cached, err := mgr.secretWatcher.Get("test/leaf")
			assert.NilError(t, err)
			assert.DeepEqual(t, cached, existing)
		})
	}
}

func TestScopeIssuance(t *testing.T) {
	ca := fixtureCASecret(t, "ca", "test")
	otherCA := fixtureCASecret(t, "other-ca", "test")
	hosts := []string{"server.example", "10.1.2.3", "2001:db8::7"}
	valid, err := certs.GenerateSecret("leaf", "server", hosts, time.Hour, ca)
	assert.NilError(t, err)
	expired, err := certs.GenerateSecret("leaf", "server", hosts, -time.Hour, ca)
	assert.NilError(t, err)
	for _, state := range []string{"absent", "missing PEM", "malformed PEM", "expired", "subject changed", "hosts changed", "valid without issuer", "older Ready generation", "deleting Certificate", "deleting Secret"} {
		t.Run(state, func(t *testing.T) {
			c := certificate("leaf", "test", "ca", "server", hosts, false, true, nil, nil)
			c.Generation = 2
			c.SetReady(nil)
			s := valid.DeepCopy()
			s.Namespace = "test"
			s.Annotations = map[string]string{"internal.skupper.io/controlled": "true", "internal.skupper.io/certificate": "true"}
			renew := true
			switch state {
			case "absent":
				s = nil
			case "missing PEM":
				delete(s.Data, "tls.crt")
			case "malformed PEM":
				s.Data["tls.crt"] = []byte("not PEM")
			case "expired":
				s.Data = expired.DeepCopy().Data
			case "subject changed":
				c.Spec.Subject = "new-server"
			case "hosts changed":
				c.Spec.Hosts = []string{"different.example", "10.2.3.4", "2001:db8::9"}
			case "valid without issuer":
				c.Spec.Ca = "missing"
				renew = false
			case "older Ready generation":
				c.Generation++
				renew = false
			case "deleting Certificate":
				c.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				c.Spec.Subject = "must not issue"
				renew = false
			case "deleting Secret":
				s.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				c.Spec.Subject = "must not issue"
				renew = false
			}
			objects := []runtime.Object{ca, otherCA}
			if s != nil {
				objects = append(objects, s)
			}
			client, err := fakeclient.NewFakeClient("test", objects, []runtime.Object{c}, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/leaf"}))
			actual, err := client.GetKubeClient().CoreV1().Secrets("test").Get(context.Background(), "leaf", metav1.GetOptions{})
			assert.NilError(t, err)
			if s == nil {
				assert.Equal(t, actual.Annotations["internal.skupper.io/certificate"], "true")
				assert.DeepEqual(t, actual.OwnerReferences, []metav1.OwnerReference{{APIVersion: "skupper.io/v2alpha1", Kind: "Certificate", Name: c.Name, UID: c.UID}})
			}
			if renew {
				decoded, err := certs.DecodeCertificate(actual.Data["tls.crt"])
				assert.NilError(t, err)
				assert.Equal(t, decoded.Subject.CommonName, c.Spec.Subject)
				// Existing issuance includes IP literals in DNSNames as well as IPAddresses.
				assert.DeepEqual(t, decoded.DNSNames, c.Spec.Hosts)
				assert.DeepEqual(t, decoded.IPAddresses, []net.IP{net.ParseIP(c.Spec.Hosts[1]).To4(), net.ParseIP(c.Spec.Hosts[2])})
				issuer, err := certs.DecodeCertificate(ca.Data["tls.crt"])
				assert.NilError(t, err)
				assert.NilError(t, decoded.CheckSignatureFrom(issuer))
				wrong, err := certs.DecodeCertificate(otherCA.Data["tls.crt"])
				assert.NilError(t, err)
				assert.Assert(t, decoded.CheckSignatureFrom(wrong) != nil)
				_, err = tls.X509KeyPair(actual.Data["tls.crt"], actual.Data["tls.key"])
				assert.NilError(t, err)
				assert.Assert(t, decoded.NotAfter.After(time.Now()))
				if s != nil {
					assert.Assert(t, !reflect.DeepEqual(actual.Data, s.Data))
				}
			} else {
				assert.DeepEqual(t, actual.Data, s.Data)
			}
			cached, err := m.secretWatcher.Get("test/leaf")
			assert.NilError(t, err)
			assert.DeepEqual(t, cached, s)
			updated, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), "leaf", metav1.GetOptions{})
			assert.NilError(t, err)
			condition := meta.FindStatusCondition(updated.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
			assert.Equal(t, condition.Status, metav1.ConditionTrue)
			assert.Equal(t, condition.ObservedGeneration, c.Generation)
			// Reconstruct the manager from API state, with no in-memory history.
			fresh := newScopeTest(t, client)
			kube := client.GetKubeClient().(*kubefake.Clientset)
			sk := client.GetSkupperClient().(*skupperfake.Clientset)
			kube.ClearActions()
			sk.ClearActions()
			assert.NilError(t, fresh.Handle(watchers.ResourceChange{Key: "test/leaf"}))
			assert.Equal(t, len(kube.Actions()), 0)
			assert.Equal(t, len(sk.Actions()), 0)
		})
	}
}

func TestSecretEventRouting(t *testing.T) {
	ctx := context.Background()
	ca := fixtureCASecret(t, "ca", "test")
	ca.Annotations = map[string]string{"internal.skupper.io/controlled": "true"}
	parent := caCertificate("ca", "test", "skupper test CA", nil, nil)
	leaf := certificate("leaf", "test", "ca", "server", nil, false, true, nil, nil)
	second := certificate("second", "test", "ca", "second", nil, false, true, nil, nil)
	otherCA := certificate("unrelated", "test", "another-ca", "other", nil, false, true, nil, nil)
	otherNS := certificate("leaf", "other", "ca", "other", nil, false, true, nil, nil)
	signing := caCertificate("unrelated-signing", "test", "self", nil, nil)
	signing.Spec.Ca = "ca"
	client, err := fakeclient.NewFakeClient("test", []runtime.Object{ca}, []runtime.Object{parent, leaf, second, otherCA, otherNS, signing}, "")
	assert.NilError(t, err)
	m := newScopeTest(t, client)
	assert.NilError(t, m.secretWatcher.Handle(watchers.ResourceChange{Key: "test/ca"}))
	m.processor.TestProcessAll()
	for _, c := range []*skupperv2alpha1.Certificate{parent, leaf, second} {
		actual, err := client.GetSkupperClient().SkupperV2alpha1().Certificates(c.Namespace).Get(ctx, c.Name, metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Assert(t, meta.IsStatusConditionTrue(actual.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY))
	}
	repaired, err := client.GetKubeClient().CoreV1().Secrets("test").Get(ctx, "ca", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, repaired, ca)
	for _, c := range []*skupperv2alpha1.Certificate{otherCA, otherNS, signing} {
		_, err := client.GetKubeClient().CoreV1().Secrets(c.Namespace).Get(ctx, c.Name, metav1.GetOptions{})
		assert.Assert(t, apierrors.IsNotFound(err))
		actual, err := client.GetSkupperClient().SkupperV2alpha1().Certificates(c.Namespace).Get(ctx, c.Name, metav1.GetOptions{})
		assert.NilError(t, err)
		assert.DeepEqual(t, actual.Status, c.Status)
	}
	// A Secret deletion observation routes the leaf scope even though it is not a CA.
	m.observe("test/leaf")
	assert.NilError(t, client.GetKubeClient().CoreV1().Secrets("test").Delete(ctx, "leaf", metav1.DeleteOptions{}))
	m.observe("test/leaf")
	assert.NilError(t, m.secretWatcher.Handle(watchers.ResourceChange{Key: "test/leaf"}))
	m.processor.TestProcessAll()
	_, err = client.GetKubeClient().CoreV1().Secrets("test").Get(ctx, "leaf", metav1.GetOptions{})
	assert.NilError(t, err)
}

func TestCertificateEventRouting(t *testing.T) {
	initial := fixtureCASecret(t, "ca", "test")
	initial.Annotations = map[string]string{"internal.skupper.io/controlled": "true", "internal.skupper.io/certificate": "true"}
	initial.OwnerReferences = []metav1.OwnerReference{{APIVersion: "skupper.io/v2alpha1", Kind: "Certificate", Name: "ca"}}
	for _, observation := range []string{"matching write", "external change", "deleted"} {
		t.Run(observation, func(t *testing.T) {
			client, err := fakeclient.NewFakeClient("test", []runtime.Object{initial.DeepCopy()}, nil, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			assert.NilError(t, m.EnsureCA("test", "ca", "issuer", nil))
			api := client.GetSkupperClient().SkupperV2alpha1().Certificates("test")
			if observation == "external change" {
				c, err := api.Get(context.Background(), "ca", metav1.GetOptions{})
				assert.NilError(t, err)
				c.Spec.Subject = "external"
				_, err = api.Update(context.Background(), c, metav1.UpdateOptions{})
				assert.NilError(t, err)
			} else if observation == "deleted" {
				assert.NilError(t, api.Delete(context.Background(), "ca", metav1.DeleteOptions{}))
			}
			m.observe("test/ca")
			assert.NilError(t, m.certificateWatcher.Handle(watchers.ResourceChange{Key: "test/ca"}))
			m.processor.TestProcessAll()
			s, err := client.GetKubeClient().CoreV1().Secrets("test").Get(context.Background(), "ca", metav1.GetOptions{})
			if observation == "deleted" {
				assert.NilError(t, err)
				assert.DeepEqual(t, s, initial) // Fake clients do not run Kubernetes GC.
			} else {
				assert.NilError(t, err)
				decoded, err := certs.DecodeCertificate(s.Data["tls.crt"])
				assert.NilError(t, err)
				subject := "issuer"
				if observation == "external change" {
					subject = "external"
				}
				assert.Equal(t, decoded.Subject.CommonName, subject)
			}
			sk := client.GetSkupperClient().(*skupperfake.Clientset)
			sk.ClearActions()
			assert.NilError(t, m.EnsureCA("test", "ca", "issuer", nil))
			if observation == "matching write" {
				assert.Equal(t, len(sk.Actions()), 0, "acknowledged authoring must use cached observation")
			}
			actual, err := api.Get(context.Background(), "ca", metav1.GetOptions{})
			assert.NilError(t, err)
			assert.Equal(t, actual.Spec.Subject, "issuer", "stale authored state must not suppress repair/recreation")
			if observation == "matching write" {
				// After acknowledgment, authoring must consult the informer, not
				// retain its old write indefinitely. Do not deliver another handler event.
				actual.Spec.Subject = "after-ack"
				_, err = api.Update(context.Background(), actual, metav1.UpdateOptions{})
				assert.NilError(t, err)
				m.observe("test/ca")
				assert.NilError(t, m.EnsureCA("test", "ca", "issuer", nil))
				actual, err = api.Get(context.Background(), "ca", metav1.GetOptions{})
				assert.NilError(t, err)
				assert.Equal(t, actual.Spec.Subject, "issuer")
			}
		})
	}
}

func TestCertificateWatchDelivery(t *testing.T) {
	ctx := context.Background()
	leaf := certificate("leaf", "test", "ca", "server", []string{"example.test", "10.1.2.3"}, false, true, nil, nil)
	client, err := fakeclient.NewFakeClient("test", nil, []runtime.Object{leaf}, "")
	assert.NilError(t, err)
	secrets := client.GetKubeClient().CoreV1().Secrets("test")
	definitions := client.GetSkupperClient().SkupperV2alpha1().Certificates("test")
	p := watchers.NewEventProcessor("cert-test", client)
	m := NewCertificateManager(p)
	m.SetControllerContext(fakeContext().control("test"))
	m.Watch(metav1.NamespaceAll)
	watchContext, stop := context.WithCancel(ctx)
	defer stop()
	defer p.Stop()
	p.StartWatchers(watchContext.Done())
	syncContext, cancelSync := context.WithTimeout(ctx, 5*time.Second)
	defer cancelSync()
	assert.Assert(t, p.WaitForCacheSync(syncContext.Done()), "Certificate and Secret initial lists did not sync")
	until := func(description string, check func() bool) {
		t.Helper()
		err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
			p.TestProcessAll()
			return check(), nil
		})
		if err != nil {
			c, _ := definitions.Get(ctx, "leaf", metav1.GetOptions{})
			s, secretErr := secrets.Get(ctx, "leaf", metav1.GetOptions{})
			t.Fatalf("waiting for %s: %v; Certificate=%+v; Secret exists=%t, error=%v", description, err, c, s != nil, secretErr)
		}
	}
	ready := func() bool {
		c, err := definitions.Get(ctx, "leaf", metav1.GetOptions{})
		return err == nil && meta.IsStatusConditionTrue(c.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
	}
	until("initial list to report missing CA", func() bool {
		c, _ := definitions.Get(ctx, "leaf", metav1.GetOptions{})
		condition := meta.FindStatusCondition(c.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
		return condition != nil && condition.Status == metav1.ConditionFalse && condition.Message == `CA "test/ca" not found`
	})
	// The CA arrives after startup and after the initial error. No Certificate
	// update or recovery call is needed to wake its dependent scope.
	_, err = secrets.Create(ctx, fixtureCASecret(t, "ca", "test"), metav1.CreateOptions{})
	assert.NilError(t, err)
	until("CA watch to issue leaf", ready)
	before, err := secrets.Get(ctx, "leaf", metav1.GetOptions{})
	assert.NilError(t, err)
	decoded, err := certs.DecodeCertificate(before.Data["tls.crt"])
	assert.NilError(t, err)
	assert.Equal(t, decoded.Subject.CommonName, "server")
	assert.NilError(t, decoded.VerifyHostname("example.test"))
	assert.NilError(t, decoded.VerifyHostname("10.1.2.3"))
	assert.DeepEqual(t, before.OwnerReferences, ownerReferences(leaf))
	assert.NilError(t, secrets.Delete(ctx, "leaf", metav1.DeleteOptions{}))
	until("Secret deletion watch to recreate leaf", func() bool {
		s, err := secrets.Get(ctx, "leaf", metav1.GetOptions{})
		return err == nil && !reflect.DeepEqual(s.Data, before.Data) && ready()
	})
}

func TestEnsureCachedAuthoring(t *testing.T) {
	ctx := context.Background()
	alice := []metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "alice", UID: "alice"}}
	bob := []metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "bob", UID: "bob"}}
	seed := certificate("leaf", "test", "ca", "server", []string{"a.example"}, false, true, nil, nil)
	seed.OwnerReferences = alice
	newOwnerMapping(alice, seed.Spec.Hosts).ApplyMetadata(seed)
	client, err := fakeclient.NewFakeClient("test", nil, []runtime.Object{seed}, "")
	assert.NilError(t, err)
	sk := client.GetSkupperClient().(*skupperfake.Clientset)
	start := func() *CertificateManagerImpl {
		return newScopeTest(t, client).CertificateManagerImpl
	}
	m := start()
	ensure := func(m *CertificateManagerImpl, hosts []string, refs []metav1.OwnerReference) error {
		return m.Ensure("test", "leaf", "ca", "server", hosts, false, true, refs)
	}
	assertNoAPI := func(m *CertificateManagerImpl, hosts []string, refs []metav1.OwnerReference) {
		t.Helper()
		sk.ClearActions()
		for i := 0; i < 3; i++ {
			assert.NilError(t, ensure(m, hosts, refs))
		}
		assert.Equal(t, len(sk.Actions()), 0, "satisfied Ensure must perform no API reads or writes")
	}
	assert.NilError(t, m.EnsureCA("test", "new-ca", "issuer", alice))
	sk.ClearActions()
	assert.NilError(t, m.EnsureCA("test", "new-ca", "issuer", alice))
	assert.Equal(t, len(sk.Actions()), 0, "successful creation must be visible before watch delivery")
	assertNoAPI(m, []string{"a.example"}, alice)
	assert.NilError(t, ensure(m, []string{"b.example"}, alice))
	assertNoAPI(m, []string{"b.example"}, alice)
	assert.NilError(t, ensure(m, []string{"a.example"}, alice))
	actual, err := sk.SkupperV2alpha1().Certificates("test").Get(ctx, "leaf", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, actual.Spec.Hosts, []string{"a.example"})
	// Deliver an older queued observation after another successful write. The
	// next call must refresh rather than treating that stale snapshot as current.
	assert.NilError(t, ensure(m, []string{"b.example"}, alice))
	assert.NilError(t, m.certificateWatcher.Handle(watchers.ResourceChange{Key: "test/leaf"}))
	assert.NilError(t, ensure(m, []string{"a.example"}, alice))
	assert.NilError(t, ensure(m, []string{"z.example"}, bob))
	actual, err = sk.SkupperV2alpha1().Certificates("test").Get(ctx, "leaf", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, actual.Spec.Hosts, []string{"a.example", "z.example"})
	assert.Equal(t, len(actual.OwnerReferences), 2)
	// Failed writes, including clients returning a non-nil result with an error,
	// cannot poison the authoring view and suppress a subsequent retry.
	fail := true
	sk.PrependReactor("update", "certificates", func(a clienttesting.Action) (bool, runtime.Object, error) {
		if fail {
			fail = false
			return true, a.(clienttesting.UpdateAction).GetObject(), fmt.Errorf("temporary write failure")
		}
		return false, nil, nil
	})
	assert.Error(t, ensure(m, []string{"c.example"}, alice), "temporary write failure")
	assert.NilError(t, ensure(m, []string{"c.example"}, alice))
	actual, err = sk.SkupperV2alpha1().Certificates("test").Get(ctx, "leaf", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, actual.Spec.Hosts, []string{"c.example", "z.example"})
	assertNoAPI(m, []string{"c.example"}, alice)
	// Deletion is another ambiguous notification. Do not retain a successful
	// write forever: refresh and recreate instead of reporting a false no-op.
	assert.NilError(t, sk.SkupperV2alpha1().Certificates("test").Delete(ctx, "leaf", metav1.DeleteOptions{}))
	assert.NilError(t, m.certificateWatcher.Handle(watchers.ResourceChange{Key: "test/leaf"}))
	assert.NilError(t, ensure(m, []string{"c.example"}, alice))
	actual, err = sk.SkupperV2alpha1().Certificates("test").Get(ctx, "leaf", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, actual.Spec.Hosts, []string{"c.example"})
	// A fresh manager has no write-through view; a synced informer suffices.
	assertNoAPI(start(), []string{"c.example"}, alice)
}

func TestExplicitSecretOwnership(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, ownerName, ownerUID           string
		controlled, marked, valid, conflict bool
	}{
		{name: "recreated parent", ownerName: "ca", ownerUID: "old", controlled: true, conflict: true},
		{name: "valid old owner", ownerName: "ca", ownerUID: "old", valid: true, conflict: true},
		{name: "foreign controlled marked owner", ownerName: "other", ownerUID: "new", controlled: true, marked: true, conflict: true},
		{name: "valid foreign marked owner", ownerName: "other", ownerUID: "new", marked: true, valid: true, conflict: true},
		{name: "matching owner", ownerName: "ca", ownerUID: "new"},
		{name: "legacy controlled", controlled: true},
		{name: "valid unmanaged", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := caCertificate("ca", "test", "skupper test CA", nil, nil)
			c.UID = "new"
			s := fixtureCASecret(t, "ca", "test")
			s.OwnerReferences, s.Annotations, s.Labels = nil, map[string]string{}, nil
			if !tc.valid {
				s.Data["tls.crt"] = []byte("invalid")
			}
			if tc.controlled {
				s.Annotations["internal.skupper.io/controlled"] = "true"
			}
			if tc.marked {
				s.Annotations["internal.skupper.io/certificate"] = "true"
			}
			if tc.ownerName != "" {
				s.OwnerReferences = ownerReferences(c)
				s.OwnerReferences[0].Name = tc.ownerName
				s.OwnerReferences[0].UID = types.UID(tc.ownerUID)
			}
			assert.Assert(t, s.DeletionTimestamp == nil)
			client, err := fakeclient.NewFakeClient("test", []runtime.Object{s}, []runtime.Object{c}, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/ca"}))
			secrets := client.GetKubeClient().CoreV1().Secrets("test")
			definitions := client.GetSkupperClient().SkupperV2alpha1().Certificates("test")
			actual, err := secrets.Get(ctx, "ca", metav1.GetOptions{})
			assert.NilError(t, err)
			observed, err := definitions.Get(ctx, "ca", metav1.GetOptions{})
			assert.NilError(t, err)
			condition := meta.FindStatusCondition(observed.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
			assert.Assert(t, condition != nil)
			if tc.conflict {
				assert.Equal(t, condition.Status, metav1.ConditionFalse)
				assert.Equal(t, condition.Reason, "Error")
				assert.Equal(t, condition.Message, "secret has conflicting Certificate ownership")
				assert.DeepEqual(t, actual, s)
				m.observe("test/ca")
				assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/ca"}))
				stable, err := definitions.Get(ctx, "ca", metav1.GetOptions{})
				assert.NilError(t, err)
				assert.DeepEqual(t, stable.Status, observed.Status)
				assert.NilError(t, secrets.Delete(ctx, "ca", metav1.DeleteOptions{}))
				m.observe("test/ca")
				assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/ca"}))
				actual, err = secrets.Get(ctx, "ca", metav1.GetOptions{})
				assert.NilError(t, err)
				assert.DeepEqual(t, actual.OwnerReferences, ownerReferences(c))
				observed, err = definitions.Get(ctx, "ca", metav1.GetOptions{})
				assert.NilError(t, err)
			} else if tc.ownerName == "" && !tc.controlled {
				assert.DeepEqual(t, actual, s)
			}
			if !tc.conflict {
				assert.Equal(t, actual.Annotations["internal.skupper.io/certificate"], s.Annotations["internal.skupper.io/certificate"], "existing Secrets must not acquire a certificate marker")
			}
			assert.Assert(t, meta.IsStatusConditionTrue(observed.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY))
			decoded, err := certs.DecodeCertificate(actual.Data["tls.crt"])
			assert.NilError(t, err)
			assert.Equal(t, decoded.Subject.CommonName, c.Spec.Subject)
		})
	}
}

func TestAbsentCertificateLeavesSecretsToGC(t *testing.T) {
	for _, tc := range []struct {
		name   string
		owners []string
		marked bool
	}{
		{name: "marker only", marked: true},
		{name: "same name old UID", owners: []string{"orphan"}},
		{name: "foreign owner overrides marker", owners: []string{"other"}, marked: true},
		{name: "mixed owners", owners: []string{"orphan", "other"}, marked: true},
		{name: "reversed mixed owners", owners: []string{"other", "orphan"}, marked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := secret("orphan", "test", nil, nil, map[string]string{})
			if tc.marked {
				s.Annotations["internal.skupper.io/certificate"] = "true"
			}
			for _, name := range tc.owners {
				s.OwnerReferences = append(s.OwnerReferences, metav1.OwnerReference{APIVersion: "skupper.io/v2alpha1", Kind: "Certificate", Name: name, UID: types.UID(name + "-old")})
			}
			client, err := fakeclient.NewFakeClient("test", []runtime.Object{s}, nil, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			kube := client.GetKubeClient().(*kubefake.Clientset)
			sk := client.GetSkupperClient().(*skupperfake.Clientset)
			kube.ClearActions()
			sk.ClearActions()
			assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/orphan"}))
			assert.Equal(t, len(kube.Actions()), 0)
			assert.Equal(t, len(sk.Actions()), 0)
			actual, err := client.GetKubeClient().CoreV1().Secrets("test").Get(context.Background(), "orphan", metav1.GetOptions{})
			assert.NilError(t, err)
			assert.DeepEqual(t, actual, s)
		})
	}
}

// Exercise the public reconciliation entry point: results must describe what
// the API actually accepted, including status writes following resource errors.
func TestActionOutcomes(t *testing.T) {
	resourceErr, statusErr := errors.New("secret write failed"), errors.New("status write failed")
	for _, tc := range []struct {
		name                     string
		existing, settled, renew bool
		resourceFail, statusFail bool
		statusUnchanged          bool
		conflict                 bool
	}{
		{name: "create and status"},
		{name: "resource failure", resourceFail: true},
		{name: "both failures", resourceFail: true, statusFail: true},
		{name: "partial success", statusFail: true},
		{name: "metadata only", existing: true},
		{name: "metadata repair already Ready", existing: true, statusUnchanged: true},
		{name: "renewal", existing: true, renew: true},
		{name: "renewal conflict", existing: true, renew: true, resourceFail: true, conflict: true},
		{name: "settled", existing: true, settled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resourceErr := resourceErr
			if tc.conflict {
				resourceErr = apierrors.NewConflict(corev1.Resource("secrets"), "ca", errors.New("stale resource version"))
			}
			c := caCertificate("ca", "test", "skupper test CA", nil, nil)
			// An initially Ready Certificate must become unready on failure,
			// even though validation would otherwise leave its status unchanged.
			if tc.resourceFail || tc.settled || tc.statusUnchanged {
				c.SetReady(nil)
			}
			if tc.renew {
				c.Spec.Subject = "replacement CA"
			}
			var objects []runtime.Object
			var original *corev1.Secret
			if tc.existing {
				original = fixtureCASecret(t, "ca", "test")
				original.Annotations = map[string]string{"internal.skupper.io/controlled": "true"}
				if tc.settled {
					original.Annotations["test-policy"] = "applied"
				}
				objects = append(objects, original)
			}
			client, err := fakeclient.NewFakeClient("test", objects, []runtime.Object{c}, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			m.SetControllerContext(fakeContext().control("test").annotate("test-policy", "applied"))
			var logs bytes.Buffer
			m.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			writes := 0
			client.GetKubeClient().(*kubefake.Clientset).PrependReactor("*", "secrets", func(a clienttesting.Action) (bool, runtime.Object, error) {
				if a.GetVerb() == "create" || a.GetVerb() == "update" || a.GetVerb() == "patch" || a.GetVerb() == "delete" {
					writes++
					if tc.resourceFail {
						return true, nil, resourceErr
					}
				}
				return false, nil, nil
			})
			client.GetSkupperClient().(*skupperfake.Clientset).PrependReactor("update", "certificates", func(a clienttesting.Action) (bool, runtime.Object, error) {
				writes++
				assert.Equal(t, a.GetSubresource(), "status")
				if tc.statusFail {
					return true, nil, statusErr
				}
				return false, nil, nil
			})
			err = m.Handle(watchers.ResourceChange{Key: "test/ca"})
			assert.Equal(t, errors.Is(err, resourceErr), tc.resourceFail)
			assert.Equal(t, errors.Is(err, statusErr), tc.statusFail)
			if !tc.resourceFail && !tc.statusFail {
				assert.NilError(t, err)
			}
			cachedCertificate, cacheErr := m.certificateWatcher.Get("test/ca")
			assert.NilError(t, cacheErr)
			assert.DeepEqual(t, cachedCertificate, c)
			cachedSecret, cacheErr := m.secretWatcher.Get("test/ca")
			assert.NilError(t, cacheErr)
			assert.DeepEqual(t, cachedSecret, original)
			actual, getErr := client.GetKubeClient().CoreV1().Secrets("test").Get(context.Background(), "ca", metav1.GetOptions{})
			if tc.resourceFail {
				if tc.existing {
					assert.NilError(t, getErr)
					assert.DeepEqual(t, actual, original)
				} else {
					assert.Assert(t, apierrors.IsNotFound(getErr))
				}
			} else {
				assert.NilError(t, getErr)
				decoded, err := certs.DecodeCertificate(actual.Data["tls.crt"])
				assert.NilError(t, err)
				assert.Equal(t, decoded.Subject.CommonName, c.Spec.Subject)
				assert.Assert(t, decoded.NotAfter.After(time.Now()))
				if tc.existing && !tc.renew {
					assert.DeepEqual(t, actual.Data, original.Data)
				}
			}
			updated, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), "ca", metav1.GetOptions{})
			assert.NilError(t, err)
			if tc.statusFail {
				assert.DeepEqual(t, updated.Status, c.Status)
			} else {
				condition := meta.FindStatusCondition(updated.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
				assert.Assert(t, condition != nil)
				assert.Equal(t, condition.Status == metav1.ConditionTrue, !tc.resourceFail)
				if tc.resourceFail {
					assert.Equal(t, condition.Message, resourceErr.Error())
				}
			}
			var outcomes []map[string]interface{}
			actionCount := -1
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var entry map[string]interface{}
				assert.NilError(t, decoder.Decode(&entry))
				if entry["action"] != nil {
					messages := map[string]map[string]string{
						"create-secret":          {"succeeded": "Created Secret for Certificate", "failed": "Failed to create Secret for Certificate"},
						"renew-secret":           {"succeeded": "Renewed Secret for Certificate", "failed": "Failed to renew Secret for Certificate"},
						"update-secret-metadata": {"succeeded": "Updated Certificate Secret metadata", "failed": "Failed to update Certificate Secret metadata"},
						"reconcile-status":       {"succeeded": "Updated Certificate status", "failed": "Failed to update Certificate status", "unchanged": "Certificate status unchanged"},
					}
					assert.Equal(t, entry["msg"], messages[entry["action"].(string)][entry["outcome"].(string)])
					assert.Equal(t, entry["namespace"], "test")
					assert.Equal(t, entry["name"], "ca")
					assert.Assert(t, entry["duration"].(float64) >= 0)
					if entry["action"] == "reconcile-status" && !tc.statusFail {
						assert.Equal(t, entry["ready"], !tc.resourceFail)
						reason, message := "Ready", "OK"
						if tc.resourceFail {
							reason, message = "Error", resourceErr.Error()
						}
						assert.Equal(t, entry["condition_reason"], reason)
						assert.Equal(t, entry["condition_message"], message)
					} else {
						_, reported := entry["ready"]
						assert.Assert(t, !reported, "only confirmed status results report Ready")
					}
					outcomes = append(outcomes, entry)
				}
				if entry["msg"] == "Reconciled certificate scope" {
					actionCount = int(entry["actions"].(float64))
					failedActions := 0
					for _, failed := range []bool{tc.resourceFail, tc.statusFail} {
						if failed {
							failedActions++
						}
					}
					assert.Equal(t, int(entry["failed_actions"].(float64)), failedActions)
					outcome := "succeeded"
					if failedActions != 0 {
						outcome = "failed"
					}
					assert.Equal(t, entry["outcome"], outcome)
					_, duplicated := entry["error"]
					assert.Assert(t, !duplicated, "scope summary must not repeat action errors")
				}
			}
			assert.Equal(t, actionCount, len(outcomes))
			if tc.settled {
				assert.Equal(t, writes, 0)
				assert.Equal(t, len(outcomes), 0)
			} else {
				expectedWrites := 2
				if tc.statusUnchanged {
					expectedWrites = 1
				}
				assert.Equal(t, writes, expectedWrites)
				assert.Equal(t, len(outcomes), 2)
				kind := "create-secret"
				if tc.existing {
					kind = "update-secret-metadata"
				}
				if tc.renew {
					kind = "renew-secret"
					assert.Equal(t, outcomes[0]["reason"], "subject-changed")
				}
				assert.Equal(t, outcomes[0]["action"], kind)
				for i, failed := range []bool{tc.resourceFail, tc.statusFail} {
					outcome := "succeeded"
					if failed {
						outcome = "failed"
					}
					if i == 1 && tc.statusUnchanged {
						outcome = "unchanged"
					}
					assert.Equal(t, outcomes[i]["outcome"], outcome)
				}
				assert.Equal(t, outcomes[len(outcomes)-1]["action"], "reconcile-status")
			}
			// Recovery is explicit, not driven by a status event or queue timer.
			// Observe any successful partial write before attempting the scope again.
			if tc.resourceFail || tc.statusFail {
				tc.resourceFail, tc.statusFail = false, false
				m.observe("test/ca")
				assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/ca"}))
				recovered, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), "ca", metav1.GetOptions{})
				assert.NilError(t, err)
				assert.Assert(t, meta.IsStatusConditionTrue(recovered.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY))
				m.observe("test/ca")
				writes = 0
				assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/ca"}))
				assert.Equal(t, writes, 0)
			}
		})
	}
}

func TestValidationDoesNotRetry(t *testing.T) {
	validationMessage := "secret exists but is not controlled by skupper"
	statusErr := errors.New("status API unavailable")
	for _, state := range []string{"new condition", "already reported", "status failure"} {
		t.Run(state, func(t *testing.T) {
			c := caCertificate("blocked", "test", "issuer", nil, nil)
			if state == "already reported" {
				c.SetReady(errors.New(validationMessage))
			}
			s := secret("blocked", "test", nil, nil, nil)
			client, err := fakeclient.NewFakeClient("test", []runtime.Object{s}, []runtime.Object{c}, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			var logs bytes.Buffer
			m.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			kube := client.GetKubeClient().(*kubefake.Clientset)
			skupper := client.GetSkupperClient().(*skupperfake.Clientset)
			if state == "status failure" {
				skupper.PrependReactor("update", "certificates", func(a clienttesting.Action) (bool, runtime.Object, error) {
					assert.Equal(t, a.GetSubresource(), "status")
					return true, nil, statusErr
				})
			}
			kube.ClearActions()
			skupper.ClearActions()
			err = m.Handle(watchers.ResourceChange{Key: "test/blocked"})
			assert.Equal(t, len(kube.Actions()), 0, "validation must not mutate the user Secret")
			var entry map[string]interface{}
			assert.NilError(t, json.NewDecoder(&logs).Decode(&entry))
			assert.Equal(t, entry["action"], "reconcile-status")
			if state == "status failure" {
				_, reported := entry["ready"]
				assert.Assert(t, !reported, "failed writes must not claim the proposed condition was recorded")
				assert.Assert(t, errors.Is(err, statusErr))
				assert.Error(t, err, statusErr.Error(), "only the API error should request retry")
				return
			}
			assert.NilError(t, err, "successfully reported validation must not request retry")
			assert.Equal(t, entry["ready"], false)
			assert.Equal(t, entry["condition_reason"], "Error")
			assert.Equal(t, entry["condition_message"], validationMessage)
			if state == "already reported" {
				assert.Equal(t, len(skupper.Actions()), 0)
			}
			actual, err := skupper.SkupperV2alpha1().Certificates("test").Get(context.Background(), "blocked", metav1.GetOptions{})
			assert.NilError(t, err)
			condition := meta.FindStatusCondition(actual.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
			assert.Assert(t, condition != nil)
			assert.Equal(t, condition.Status, metav1.ConditionFalse)
			assert.Equal(t, condition.Reason, "Error")
			assert.Equal(t, condition.Message, validationMessage)
		})
	}
}

func TestScopeNamespaceControl(t *testing.T) {
	for _, namespace := range []string{"controlled", "uncontrolled"} {
		t.Run(namespace, func(t *testing.T) {
			c := caCertificate("ca", namespace, "issuer", nil, nil)
			client, err := fakeclient.NewFakeClient(namespace, nil, []runtime.Object{c}, "")
			assert.NilError(t, err)
			m := newScopeTest(t, client)
			m.SetControllerContext(fakeContext().control("controlled").label("foo", "bar").annotate("x", "y"))
			assert.NilError(t, m.Handle(watchers.ResourceChange{Key: c.Key()}))
			secrets := client.GetKubeClient().CoreV1().Secrets(namespace)
			definitions := client.GetSkupperClient().SkupperV2alpha1().Certificates(namespace)
			s, secretErr := secrets.Get(context.Background(), "ca", metav1.GetOptions{})
			actual, err := definitions.Get(context.Background(), "ca", metav1.GetOptions{})
			assert.NilError(t, err)
			if namespace == "uncontrolled" {
				assert.Assert(t, apierrors.IsNotFound(secretErr))
				assert.DeepEqual(t, actual, c)
				return
			}
			assert.NilError(t, secretErr)
			assert.Equal(t, s.Labels["foo"], "bar")
			assert.Equal(t, s.Annotations["x"], "y")
			assert.Assert(t, meta.IsStatusConditionTrue(actual.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY))
			// Parent deletion leaves the Secret to Kubernetes GC, which fake
			// clients do not run.
			m.observe(c.Key())
			assert.NilError(t, definitions.Delete(context.Background(), "ca", metav1.DeleteOptions{}))
			m.observe(c.Key())
			assert.NilError(t, m.Handle(watchers.ResourceChange{Key: c.Key()}))
			retained, err := secrets.Get(context.Background(), "ca", metav1.GetOptions{})
			assert.NilError(t, err)
			assert.DeepEqual(t, retained, s)
		})
	}
}

func TestOwnerContributionPruning(t *testing.T) {
	alice := metav1.OwnerReference{Name: "alice", UID: "alice"}
	bob := metav1.OwnerReference{Name: "bob", UID: "bob"}
	c := certificate("leaf", "test", "ca", "server", []string{"aaa", "bbb", "ccc"}, false, true, nil, nil)
	managedWithOwnerHosts(t, c, alice, "aaa", "bbb")
	managedWithOwnerHosts(t, c, bob, "bbb", "ccc")
	c.OwnerReferences = []metav1.OwnerReference{alice}
	client, err := fakeclient.NewFakeClient("test", nil, []runtime.Object{c}, "")
	assert.NilError(t, err)
	m := newScopeTest(t, client)
	assert.NilError(t, m.Handle(watchers.ResourceChange{Key: c.Key()}))
	actual, err := client.GetSkupperClient().SkupperV2alpha1().Certificates("test").Get(context.Background(), "leaf", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, actual.Spec.Hosts, []string{"aaa", "bbb"})
	assert.DeepEqual(t, actual.OwnerReferences, []metav1.OwnerReference{alice})
	assert.Equal(t, actual.Annotations["internal.skupper.io/hosts-alice"], "aaa,bbb")
	_, retained := actual.Annotations["internal.skupper.io/hosts-bob"]
	assert.Assert(t, !retained)
	assert.DeepEqual(t, actual.Status, c.Status)
}

func TestOwnerCleanupAwaitsObservation(t *testing.T) {
	c := managedWithOwnerHosts(t, certificate("ca", "test", "issuer", "server", []string{"obsolete.example"}, false, true, nil, nil), metav1.OwnerReference{UID: "gone"}, "obsolete.example")
	c.OwnerReferences = nil
	client, err := fakeclient.NewFakeClient("test", []runtime.Object{fixtureCASecret(t, "issuer", "test")}, []runtime.Object{c}, "")
	assert.NilError(t, err)
	m := newScopeTest(t, client)
	var logs bytes.Buffer
	m.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	kube := client.GetKubeClient().(*kubefake.Clientset)
	skupper := client.GetSkupperClient().(*skupperfake.Clientset)
	kube.ClearActions()
	skupper.ClearActions()
	assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/ca"}))
	var entry map[string]interface{}
	assert.NilError(t, json.NewDecoder(&logs).Decode(&entry))
	assert.Equal(t, entry["action"], "cleanup-certificate")
	assert.Equal(t, entry["reason"], "owner-contributions-changed")
	assert.Equal(t, len(kube.Actions()), 0, "cleanup must not issue a Secret from the stale Certificate")
	actions := skupper.Actions()
	assert.Equal(t, len(actions), 1, "cleanup must not write status from the stale Certificate")
	assert.Equal(t, actions[0].GetVerb(), "update")
	assert.Equal(t, actions[0].GetSubresource(), "")
	updated, err := skupper.SkupperV2alpha1().Certificates("test").Get(context.Background(), "ca", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(updated.Spec.Hosts), 0)
	assert.DeepEqual(t, updated.Status, c.Status)
	cached, err := m.certificateWatcher.Get("test/ca")
	assert.NilError(t, err)
	assert.DeepEqual(t, cached, c)
	m.observe("test/ca")
	assert.NilError(t, m.Handle(watchers.ResourceChange{Key: "test/ca"}))
	issued, err := kube.CoreV1().Secrets("test").Get(context.Background(), "ca", metav1.GetOptions{})
	assert.NilError(t, err)
	decoded, err := certs.DecodeCertificate(issued.Data["tls.crt"])
	assert.NilError(t, err)
	assert.Equal(t, len(decoded.DNSNames), 0)
}

func TestCertificatePlan(t *testing.T) {
	c := caCertificate("ca", "test", "skupper test CA", nil, nil)
	c.SetReady(nil)
	s := fixtureCASecret(t, "ca", "test")
	s.Annotations = map[string]string{"internal.skupper.io/controlled": "true", "internal.skupper.io/certificate": "true"}
	decoded, err := certs.DecodeCertificate(s.Data["tls.crt"])
	assert.NilError(t, err)
	for _, tc := range []struct {
		delta   time.Duration
		actions []certificateActionKind
	}{{-time.Nanosecond, nil}, {0, nil}, {time.Nanosecond, []certificateActionKind{renewSecret, reconcileStatus}}} {
		snapshot := certificateSnapshot{certificate: c, secret: s, now: decoded.NotAfter.Add(tc.delta)}
		originalCert, originalSecret := c.DeepCopy(), s.DeepCopy()
		first, second := planCertificate(snapshot), planCertificate(snapshot)
		assert.Assert(t, reflect.DeepEqual(first, second), "identical snapshots must produce identical plans")
		assert.Equal(t, len(first), len(tc.actions))
		for i, kind := range tc.actions {
			assert.Equal(t, first[i].kind, kind)
		}
		if tc.delta > 0 {
			assert.Equal(t, first[0].reason, "expired")
		}
		assert.DeepEqual(t, c, originalCert)
		assert.DeepEqual(t, s, originalSecret)
	}
}
