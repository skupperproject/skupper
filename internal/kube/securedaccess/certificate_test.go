package securedaccess

import (
	"context"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/kube/watchers"
	saapi "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReservedCertificateGeneration(t *testing.T) {
	for _, name := range []string{"skupper-site-ca", "skupper-local-ca", "skupper-local-server", "skupper-grant-server-ca"} {
		for _, tc := range []struct {
			name     string
			fallback bool
		}{{name: "explicit"}, {name: "fallback", fallback: true}} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				h := newHarness(t, nil)
				ctx := context.Background()
				sa := testAccess("local")
				sa.Spec.Issuer = "skupper-site-ca" // Reserved issuers are valid.
				sa.Spec.Certificate = name
				if tc.fallback {
					sa.Name, sa.Spec.Certificate = name, ""
				}
				h.put(sa)
				// Even a legacy Access-owned reserved output is never adopted,
				// changed, or removed by the aggregate writer.
				original := &saapi.Certificate{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", OwnerReferences: ownerReferences(sa),
						Annotations: map[string]string{"internal.skupper.io/controlled": "true"}},
					Spec: saapi.CertificateSpec{Subject: "infrastructure", Signing: true},
				}
				_, err := h.clients.Skupper.SkupperV2alpha1().Certificates("test").Create(ctx, original, metav1.CreateOptions{})
				assert.NilError(t, err)
				h.observe()
				assert.NilError(t, h.run(sa.Key()))
				assert.Assert(t, strings.Contains(h.status(sa.Name).Status.Message, "set spec.certificate to a non-reserved name"))
				assert.Assert(t, !h.status(sa.Name).IsReady())
				assert.ErrorContains(t, h.m.certificates.Handle(watchers.ResourceChange{Key: "test/" + name}), "reserved")
				// Without an issuer this is consumption of an existing Secret,
				// not a request to author Certificate intent.
				sa = h.status(sa.Name)
				sa.Spec.Issuer = ""
				_, err = h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Update(ctx, sa, metav1.UpdateOptions{})
				assert.NilError(t, err)
				h.settle(sa.Name)
				assert.Assert(t, h.status(sa.Name).IsReady())
				assert.NilError(t, h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Delete(ctx, sa.Name, metav1.DeleteOptions{}))
				h = newHarness(t, h.clients)
				h.observe()
				assert.NilError(t, h.runCertificates())
				got, err := h.clients.Skupper.SkupperV2alpha1().Certificates("test").Get(ctx, name, metav1.GetOptions{})
				assert.NilError(t, err)
				assert.DeepEqual(t, got.Spec, original.Spec)
				assert.DeepEqual(t, got.OwnerReferences, original.OwnerReferences)
			})
		}
	}
}

func TestSharedCertificateIssuerConflictAndRecovery(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	a := testAccess("local")
	a.Spec.Issuer, a.Spec.Certificate = "skupper-site-ca", "shared"
	h.put(a)
	h.settle(a.Name)
	before, err := h.clients.Skupper.SkupperV2alpha1().Certificates("test").Get(ctx, "shared", metav1.GetOptions{})
	assert.NilError(t, err)
	b := a.DeepCopy()
	b.Name, b.UID, b.Spec.Issuer = "second", "second-uid", "different-ca"
	h.put(b)
	for i := 0; i < 2; i++ {
		assert.NilError(t, h.run(a.Key()))
		assert.NilError(t, h.run(b.Key()))
		assert.Assert(t, strings.Contains(h.status(a.Name).Status.Message, "incompatible SecuredAccess issuers"))
		assert.Assert(t, strings.Contains(h.status(b.Name).Status.Message, "incompatible SecuredAccess issuers"))
		assert.ErrorContains(t, h.runCertificates(), "incompatible SecuredAccess issuers")
		h.observe()
	}
	assert.Assert(t, !h.status(a.Name).IsReady())
	assert.Assert(t, !h.status(b.Name).IsReady())
	got, err := h.clients.Skupper.SkupperV2alpha1().Certificates("test").Get(ctx, "shared", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, got, before)
	assert.NilError(t, h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Delete(ctx, b.Name, metav1.DeleteOptions{}))
	h.settle(a.Name)
	assert.Assert(t, h.status(a.Name).IsReady())
	h.noWrites(a.Name)
}

func TestForeignCertificateNotAdoptedOrDeleted(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mixed bool
	}{{name: "foreign"}, {name: "mixed", mixed: true}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, nil)
			ctx := context.Background()
			sa := testAccess("local")
			sa.Spec.Issuer, sa.Spec.Certificate = "issuer", "shared"
			h.put(sa)
			refs := []metav1.OwnerReference{{APIVersion: "skupper.io/v2alpha1", Kind: "Site", Name: "site", UID: "site-uid"}}
			if tc.mixed {
				refs = append(refs, ownerReferences(sa)...)
			}
			original := &saapi.Certificate{
				ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "test", OwnerReferences: refs,
					Annotations: map[string]string{"internal.skupper.io/controlled": "true"}},
				Spec: saapi.CertificateSpec{Subject: "foreign", Hosts: []string{"foreign.example"}, Ca: "foreign-ca"},
			}
			_, err := h.clients.Skupper.SkupperV2alpha1().Certificates("test").Create(ctx, original, metav1.CreateOptions{})
			assert.NilError(t, err)
			h.observe()
			assert.NilError(t, h.run(sa.Key()))
			assert.Assert(t, strings.Contains(h.status(sa.Name).Status.Message, "conflicting ownership"))
			assert.ErrorContains(t, h.runCertificates(), "conflicting ownership")
			assert.NilError(t, h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Delete(ctx, sa.Name, metav1.DeleteOptions{}))
			h = newHarness(t, h.clients)
			h.observe()
			assert.NilError(t, h.runCertificates())
			got, err := h.clients.Skupper.SkupperV2alpha1().Certificates("test").Get(ctx, "shared", metav1.GetOptions{})
			assert.NilError(t, err)
			assert.DeepEqual(t, got.Spec, original.Spec)
			assert.DeepEqual(t, got.OwnerReferences, refs)
		})
	}
}

func TestCertificatePendingHostsAndSurvivingTargets(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	sa := testAccess("ingress")
	sa.Spec.Issuer, sa.Spec.Certificate = "issuer", "shared"
	h.put(sa)
	h.settle(sa.Name)
	certs := h.clients.Skupper.SkupperV2alpha1().Certificates("test")
	before, err := certs.Get(ctx, "shared", metav1.GetOptions{})
	assert.NilError(t, err)
	sa = h.status(sa.Name)
	sa.Generation++
	sa.Status.Conditions = nil
	_, err = h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Update(ctx, sa, metav1.UpdateOptions{})
	assert.NilError(t, err)
	h.observe()
	assert.NilError(t, h.runCertificates())
	got, err := certs.Get(ctx, "shared", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, got, before)
	sa.Spec.Certificate = "replacement"
	_, err = h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Update(ctx, sa, metav1.UpdateOptions{})
	assert.NilError(t, err)
	// A fresh manager must derive both old and current aggregate intent. Actual
	// informer routing of surviving targets is covered in controller tests.
	h = newHarness(t, h.clients)
	h.m.config.EnabledAccessTypes = []string{"local", "ingress"}
	h.observe()
	assert.NilError(t, h.runCertificates())
	_, err = certs.Get(ctx, "shared", metav1.GetOptions{})
	assert.Assert(t, apierrors.IsNotFound(err))
	got, err = certs.Get(ctx, "replacement", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, got.Spec.Ca, "issuer")
	sa = h.status(sa.Name)
	sa.Spec.Issuer = ""
	_, err = h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Update(ctx, sa, metav1.UpdateOptions{})
	assert.NilError(t, err)
	h.observe()
	assert.NilError(t, h.runCertificates())
	_, err = certs.Get(ctx, "replacement", metav1.GetOptions{})
	assert.Assert(t, apierrors.IsNotFound(err))
}

func TestCertificatePolicyPreservesAuthority(t *testing.T) {
	h := newHarness(t, nil)
	policy := h.m.context.(*testContext).policy
	template := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "controller", Name: "certificate-authority",
		Labels:      map[string]string{"skupper.io/label-template": "true"},
		Annotations: map[string]string{"internal.skupper.io/controlled": "false", "internal.skupper.io/secured-access-certificate": "false", "decoration": "created"},
	}, Data: map[string]string{"kind": "Certificate", "name": "shared"}}
	assert.NilError(t, policy.Update("controller/certificate-authority", template))
	sa := testAccess("local")
	sa.Spec.Issuer, sa.Spec.Certificate = "issuer", "shared"
	h.put(sa)
	h.settle(sa.Name)
	assert.Assert(t, h.status(sa.Name).IsReady())
	certs := h.clients.Skupper.SkupperV2alpha1().Certificates("test")
	created, err := certs.Get(context.Background(), "shared", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, created.Annotations["internal.skupper.io/controlled"], "true")
	assert.Equal(t, created.Annotations["internal.skupper.io/secured-access-certificate"], "true")
	assert.Equal(t, created.Annotations["decoration"], "created")

	template = template.DeepCopy()
	template.Annotations["decoration"] = "updated"
	assert.NilError(t, policy.Update("controller/certificate-authority", template))
	h.settle(sa.Name)
	updated, err := certs.Get(context.Background(), "shared", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, updated.UID, created.UID)
	assert.Equal(t, updated.Annotations["decoration"], "updated")
	assert.Equal(t, updated.Annotations["internal.skupper.io/controlled"], "true")
	assert.Equal(t, updated.Annotations["internal.skupper.io/secured-access-certificate"], "true")

	h = newHarness(t, h.clients)
	h.m.context.(*testContext).policy = policy
	h.settle(sa.Name)
	assert.Assert(t, h.status(sa.Name).IsReady())
	assert.NilError(t, policy.Update("controller/certificate-authority", nil))
	h.settle(sa.Name)
	assert.Assert(t, h.status(sa.Name).IsReady())
	h.noWrites(sa.Name)
	assert.NilError(t, h.clients.Skupper.SkupperV2alpha1().SecuredAccesses("test").Delete(context.Background(), sa.Name, metav1.DeleteOptions{}))
	h.observe()
	assert.NilError(t, h.runCertificates())
	_, err = certs.Get(context.Background(), "shared", metav1.GetOptions{})
	assert.Assert(t, apierrors.IsNotFound(err))
}
