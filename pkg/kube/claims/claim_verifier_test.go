package claims

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/event"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestClientContext struct {
	KubeClient *internalclient.KubeClient
	Namespace  string
}

func (*TestClientContext) VerifySiteCompatibility(siteVersion string) error {
	if siteVersion == "this-site-is-no-good" {
		return fmt.Errorf("Incompatible site")
	}
	return nil
}

func createClaimRecord(cli *internalclient.KubeClient, name string, password []byte, expiration *time.Time, uses int) error {
	record := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRecord,
			},
			Annotations: map[string]string{},
		},
		Data: map[string][]byte{
			types.ClaimPasswordDataKey: password,
		},
	}
	if expiration != nil {
		record.ObjectMeta.Annotations[types.ClaimExpiration] = expiration.Format(time.RFC3339)
	}
	if uses > 0 {
		record.ObjectMeta.Annotations[types.ClaimsRemaining] = strconv.Itoa(uses)
	}
	_, err := cli.GetKubeClient().CoreV1().Secrets(cli.Namespace).Create(context.TODO(), &record, metav1.CreateOptions{})
	return err
}

type MockTokenGenerator struct {
	Secret *corev1.Secret
	Error  error
}

func newMockTokenGenerator(err error) *MockTokenGenerator {
	return &MockTokenGenerator{
		Error: err,
		Secret: &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "placeholder",
			},
		},
	}
}

func (o *MockTokenGenerator) ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error) {
	o.Secret.ObjectMeta.Name = subject
	return o.Secret, false, o.Error
}

func TestClaimVerifier(t *testing.T) {

	event.StartDefaultEventStore(nil)
	cli, err := fakeclient.NewFakeClient("claim-verifier-test", nil, nil, "")
	assert.Assert(t, err)
	tc := &TestClientContext{
		KubeClient: cli,
	}

	generator := newMockTokenGenerator(nil)
	verifier := newClaimVerifier(cli.GetKubeClient(), cli.Namespace, generator, tc)

	//create some claim records
	err = createClaimRecord(cli, "a", []byte("abcdefg"), nil, 2)
	assert.Check(t, err, "claim-verifier-test: creating a")
	expiration := time.Now().Add(-1 * time.Hour)
	err = createClaimRecord(cli, "b", []byte("abcdefg"), &expiration, 1)
	assert.Check(t, err, "claim-verifier-test: creating b")

	//simple test of valid claim
	secret, _, code := verifier.redeemClaim("a", "foo", []byte("abcdefg"), generator)
	assert.Equal(t, code, http.StatusOK, "claim-verifier-test: a")
	assert.Equal(t, secret, generator.Secret, "claim-verifier-test: a")
	assert.Equal(t, secret.ObjectMeta.Name, "foo", "claim-verifier-test: a")
	record, err := cli.GetKubeClient().CoreV1().Secrets(cli.Namespace).Get(context.TODO(), "a", metav1.GetOptions{})
	assert.Check(t, err, "claim-verifier-test: a")
	assert.Equal(t, record.ObjectMeta.Annotations[types.ClaimsRemaining], "1", "claim-verifier-test: a")
	assert.Equal(t, record.ObjectMeta.Annotations[types.ClaimsMade], "1", "claim-verifier-test: a")

	//test password checking
	secret, _, code = verifier.redeemClaim("a", "foo", []byte("blahblah"), generator)
	assert.Equal(t, code, http.StatusForbidden, "claim-verifier-test: a, bad password")
	assert.Assert(t, secret == nil, "claim-verifier-test: a, bad password")

	secret, _, code = verifier.redeemClaim("a", "foo", []byte("abcdefg"), generator)
	assert.Equal(t, code, http.StatusOK, "claim-verifier-test: a 2nd attempt")
	assert.Equal(t, secret, generator.Secret, "claim-verifier-test: a 2nd attempt")
	assert.Equal(t, secret.ObjectMeta.Name, "foo", "claim-verifier-test: a 2nd attempt")
	record, err = cli.GetKubeClient().CoreV1().Secrets(cli.Namespace).Get(context.TODO(), "a", metav1.GetOptions{})
	assert.Equal(t, record.ObjectMeta.Annotations[types.ClaimsRemaining], "0", "claim-verifier-test: a")
	assert.Equal(t, record.ObjectMeta.Annotations[types.ClaimsMade], "2", "claim-verifier-test: a")

	//test claim that does not exist
	secret, _, code = verifier.redeemClaim("not-there", "foo", []byte("abcdefg"), generator)
	//  - check the result is as expected
	assert.Equal(t, code, http.StatusNotFound, "claim-verifier-test: not-there")
	assert.Assert(t, secret == nil, "claim-verifier-test: not-there")

	//test expired claim
	secret, _, code = verifier.redeemClaim("b", "foo", []byte("abcdefg"), generator)
	assert.Equal(t, code, http.StatusNotFound, "claim-verifier-test: b")
	assert.Assert(t, secret == nil, "claim-verifier-test: b")
}

func TestServeClaims(t *testing.T) {
	event.StartDefaultEventStore(nil)
	var tests = []struct {
		name         string
		method       string
		path         string
		body         io.Reader
		expectedCode int
	}{
		{
			method:       http.MethodGet,
			path:         "/",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodPost,
			path:         "/myclaim",
			body:         bytes.NewBufferString("abcdefg"),
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodPost,
			path:         "/anotherclaim",
			body:         bytes.NewBufferString("abcdefg"),
			expectedCode: http.StatusForbidden,
		},
		{
			method:       http.MethodPost,
			path:         "/doesntexist",
			body:         bytes.NewBufferString("abcdefg"),
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodPost,
			path:         "/incompatible?site-version=this-site-is-no-good",
			body:         bytes.NewBufferString("abcdefg"),
			expectedCode: http.StatusBadRequest,
		},
	}
	cli, err := fakeclient.NewFakeClient("server-claims-test", nil, nil, "")
	assert.Assert(t, err)
	tc := &TestClientContext{
		KubeClient: cli,
	}
	generator := newMockTokenGenerator(nil)
	verifier := newClaimVerifier(cli.GetKubeClient(), cli.Namespace, generator, tc)
	err = createClaimRecord(cli, "myclaim", []byte("abcdefg"), nil, 1)
	assert.Check(t, err, "serve-claims-test: creating mytoken")
	err = createClaimRecord(cli, "anotherclaim", []byte("password"), nil, 1)
	assert.Check(t, err, "serve-claims-test: creating anothertoken")
	for _, test := range tests {
		name := test.name
		if name == "" {
			name = test.method + " " + test.path
		}
		req := httptest.NewRequest(test.method, test.path, test.body)
		res := httptest.NewRecorder()

		verifier.ServeHTTP(res, req)
		assert.Equal(t, res.Code, test.expectedCode, name)
	}
}
