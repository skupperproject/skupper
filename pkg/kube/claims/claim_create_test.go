package claims

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	testingk8s "k8s.io/client-go/testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
)

type ClaimCreateTestContext struct {
	edge           bool
	siteId         string
	siteVersion    string
	ownerRefs      []metav1.OwnerReference
	claimsHostPort resolver.HostPort
	err            error
}

func TestCreateTokenClaim(t *testing.T) {
	event.StartDefaultEventStore(nil)
	var tests = []struct {
		name        string
		createCA    bool
		ctxt        ClaimCreateTestContext
		password    []byte
		expiration  time.Duration
		uses        int
		createError error
	}{
		{
			name:     "foo",
			createCA: true,
			ctxt: ClaimCreateTestContext{
				claimsHostPort: resolver.HostPort{
					Host: "myhost",
					Port: 123,
				},
				siteVersion: "myversion",
				siteId:      "mysite",
			},
			password:   []byte("mypassword"),
			expiration: 0,
			uses:       2,
		},
		{
			name:     "bar",
			createCA: true,
			ctxt: ClaimCreateTestContext{
				claimsHostPort: resolver.HostPort{
					Host: "differenthost",
					Port: 678,
				},
				siteVersion: "anotherversion",
				siteId:      "anothersite",
			},
			password:   []byte("hardtoguess?"),
			expiration: 10 * time.Minute,
			uses:       1,
		},
		{
			name:     "",
			createCA: true,
			ctxt: ClaimCreateTestContext{
				claimsHostPort: resolver.HostPort{
					Host: "acme.com",
					Port: 987,
				},
				siteVersion: "version2",
				siteId:      "thisismysite",
			},
			password: []byte("simples"),
			uses:     1,
		},
		{
			name:     "",
			createCA: true,
			ctxt: ClaimCreateTestContext{
				claimsHostPort: resolver.HostPort{
					Host: "acme.com",
					Port: 987,
				},
				siteVersion: "version2",
				siteId:      "thisismysite",
				err:         fmt.Errorf("Failed to get host-port"),
			},
			password: []byte("simples"),
			uses:     1,
		},
		{
			name:     "failure1",
			createCA: false,
			ctxt: ClaimCreateTestContext{
				claimsHostPort: resolver.HostPort{
					Host: "acme.com",
					Port: 987,
				},
				siteVersion: "version2",
				siteId:      "thisismysite",
			},
			password: []byte("simples"),
			uses:     1,
		},
		{
			name:     "failure2",
			createCA: true,
			ctxt: ClaimCreateTestContext{
				edge: true,
				claimsHostPort: resolver.HostPort{
					Host: "acme.com",
					Port: 987,
				},
				siteVersion: "version2",
				siteId:      "thisismysite",
				err:         fmt.Errorf("Failed to get host-port"),
			},
			password: []byte("simples"),
			uses:     1,
		},
		{
			name:     "failure3",
			createCA: true,
			ctxt: ClaimCreateTestContext{
				claimsHostPort: resolver.HostPort{
					Host: "acme.com",
					Port: 987,
				},
				siteVersion: "version2",
				siteId:      "thisismysite",
				err:         fmt.Errorf("Failed to get host-port"),
			},
			password: []byte("simples"),
			uses:     1,
		},
		{
			name:        "failure4",
			createCA:    true,
			createError: fmt.Errorf("Failed to create secret"),
			ctxt: ClaimCreateTestContext{
				claimsHostPort: resolver.HostPort{
					Host: "acme.com",
					Port: 987,
				},
				siteVersion: "version2",
				siteId:      "thisismysite",
			},
			password: []byte("simples"),
			uses:     1,
		},
	}
	for _, test := range tests {
		cli := &TestClientContext{
			Namespace:  "create-token-claim-" + test.name,
			KubeClient: fake.NewSimpleClientset(),
		}
		ctxt := &test.ctxt
		if test.createCA {
			err := cli.createCA(types.SiteCaSecret)
			assert.Check(t, err, "claim-verifier-test: creating CA")
		}
		factory := NewClaimFactory(cli, cli.Namespace, ctxt, context.TODO())
		if test.createError != nil {
			cli.KubeClient.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("create", "secrets", func(action testingk8s.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, test.createError
			})
		}
		token, err := factory.CreateTokenClaim(test.name, test.password, test.expiration, test.uses)
		if test.createError != nil {
			assert.Equal(t, err, test.createError)
		} else if !test.createCA || test.ctxt.edge {
			assert.Assert(t, err != nil, "Expected error")
		} else if test.ctxt.err != nil {
			assert.Equal(t, err, test.ctxt.err)
		} else {
			//should succeed
			assert.Check(t, err, "claim-verifier-test: creating foo")
			//check token:
			assert.Assert(t, token != nil)
			assert.Equal(t, token.Labels[types.SkupperTypeQualifier], types.TypeClaimRequest)
			assert.Equal(t, token.Annotations[types.SiteVersion], ctxt.siteVersion)
			assert.Equal(t, token.Annotations[types.TokenGeneratedBy], ctxt.siteId)
			assert.Assert(t, bytes.Equal(token.Data[types.ClaimPasswordDataKey], test.password))
			if test.name != "" {
				assert.Equal(t, token.Annotations[types.ClaimUrlAnnotationKey], fmt.Sprintf("https://%s:%d/%s", ctxt.claimsHostPort.Host, ctxt.claimsHostPort.Port, test.name))
			} else {
				assert.Assert(t, strings.HasPrefix(token.Annotations[types.ClaimUrlAnnotationKey], fmt.Sprintf("https://%s:%d/", ctxt.claimsHostPort.Host, ctxt.claimsHostPort.Port)))
			}
			//check claim record:
			u, _ := url.Parse(token.Annotations[types.ClaimUrlAnnotationKey])
			name := strings.Join(strings.Split(u.Path, "/"), "")
			record, err := cli.GetKubeClient().CoreV1().Secrets(cli.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
			assert.Check(t, err, "claim-verifier-test: checking claim record")
			assert.Equal(t, record.Labels[types.SkupperTypeQualifier], types.TypeClaimRecord)
			assert.Equal(t, record.Annotations[types.SiteVersion], ctxt.siteVersion)
			assert.Equal(t, record.Annotations[types.ClaimsRemaining], strconv.Itoa(test.uses))
			assert.Assert(t, bytes.Equal(record.Data[types.ClaimPasswordDataKey], test.password))
		}
	}
}

func TestRecreateTokenClaim(t *testing.T) {
	event.StartDefaultEventStore(nil)
	var tests = []struct {
		name         string
		noClaim      bool
		secretExists bool
		failGet      bool
		password     []byte
		ctxt         ClaimCreateTestContext
	}{
		{
			name:     "claim1",
			password: []byte("abcdefg"),
		},
		{
			name:     "claim2",
			noClaim:  true,
			password: []byte("abcdefg"),
		},
		{
			name:         "claim3",
			noClaim:      true,
			secretExists: true,
			password:     []byte("abcdefg"),
		},
		{
			name:     "claim3",
			failGet:  true,
			password: []byte("abcdefg"),
		},
	}
	for _, test := range tests {
		cli := &TestClientContext{
			Namespace:  "recreate-token-claim-" + test.name,
			KubeClient: fake.NewSimpleClientset(),
		}
		ctxt := &test.ctxt
		err := cli.createCA(types.SiteCaSecret)
		assert.Check(t, err)
		factory := NewClaimFactory(cli, cli.Namespace, ctxt, context.TODO())
		if test.noClaim {
			if test.secretExists {
				cli.createSecret(test.name)
			}
			token, err := factory.RecreateTokenClaim(test.name)
			assert.Check(t, err)
			assert.Assert(t, token == nil)
		} else if test.failGet {
			expected := fmt.Errorf("error getting secret")
			cli.KubeClient.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("get", "secrets", func(action testingk8s.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, expected
			})
			_, err := factory.RecreateTokenClaim(test.name)
			assert.Equal(t, err, expected)
		} else {
			original, err := factory.CreateTokenClaim(test.name, test.password, 0, 1)
			assert.Check(t, err)
			token, err := factory.RecreateTokenClaim(test.name)
			assert.Check(t, err)
			assert.Assert(t, bytes.Equal(token.Data[types.ClaimPasswordDataKey], test.password))
			assert.Assert(t, bytes.Equal(token.Data[types.ClaimCaCertDataKey], original.Data[types.ClaimCaCertDataKey]))
			assert.Equal(t, token.Labels[types.SkupperTypeQualifier], original.Labels[types.SkupperTypeQualifier])
			assert.Equal(t, token.Annotations[types.SiteVersion], original.Annotations[types.SiteVersion])
			assert.Equal(t, token.Annotations[types.TokenGeneratedBy], original.Annotations[types.TokenGeneratedBy])
			assert.Equal(t, token.Annotations[types.ClaimUrlAnnotationKey], original.Annotations[types.ClaimUrlAnnotationKey])
		}
	}
}

func (c *ClaimCreateTestContext) IsLocalAccessOnly() bool {
	return false
}

func (c *ClaimCreateTestContext) GetAllHosts() ([]string, error) {
	return nil, nil
}

func (c *ClaimCreateTestContext) GetHostPortForInterRouter() (resolver.HostPort, error) {
	return resolver.HostPort{}, nil
}

func (c *ClaimCreateTestContext) GetHostPortForEdge() (resolver.HostPort, error) {
	return resolver.HostPort{}, nil
}

func (c *ClaimCreateTestContext) GetHostPortForClaims() (resolver.HostPort, error) {
	return c.claimsHostPort, c.err
}

func (c *ClaimCreateTestContext) IsEdge() bool {
	return c.edge
}

func (c *ClaimCreateTestContext) GetSiteVersion() string {
	return c.siteVersion
}

func (c *ClaimCreateTestContext) GetSiteId() string {
	return c.siteId
}

func (c *ClaimCreateTestContext) GetOwnerReferences() []metav1.OwnerReference {
	return c.ownerRefs
}

func (c *TestClientContext) createCA(name string) error {
	newCA := certs.GenerateCASecret(name, name)
	_, err := c.GetKubeClient().CoreV1().Secrets(c.Namespace).Create(context.TODO(), &newCA, metav1.CreateOptions{})
	return err
}

func (c *TestClientContext) createSecret(name string) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := c.GetKubeClient().CoreV1().Secrets(c.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	return err
}
