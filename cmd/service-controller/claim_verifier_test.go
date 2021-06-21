package main

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func createClaimRecord(cli *client.VanClient, name string, password []byte, expiration *time.Time, uses int) error {
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
	_, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(&record)
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
	cli := &client.VanClient{
		Namespace:  "claim-verifier-test",
		KubeClient: fake.NewSimpleClientset(),
	}

	verifier := newClaimVerifier(cli)
	generator := newMockTokenGenerator(nil)

	//create some claim records
	err := createClaimRecord(cli, "a", []byte("abcdefg"), nil, 2)
	assert.Check(t, err, "claim-verifier-test: creating a")
	expiration := time.Now().Add(-1 * time.Hour)
	err = createClaimRecord(cli, "b", []byte("abcdefg"), &expiration, 1)
	assert.Check(t, err, "claim-verifier-test: creating b")

	//simple test of valid claim
	secret, _, code := verifier.redeemClaim("a", "foo", []byte("abcdefg"), generator)
	assert.Equal(t, code, http.StatusOK, "claim-verifier-test: a")
	assert.Equal(t, secret, generator.Secret, "claim-verifier-test: a")
	assert.Equal(t, secret.ObjectMeta.Name, "foo", "claim-verifier-test: a")
	record, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get("a", metav1.GetOptions{})
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
	record, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get("a", metav1.GetOptions{})
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
