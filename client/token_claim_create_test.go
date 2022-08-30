package client

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

func readSecretFromFile(filename string) (*corev1.Secret, error) {
	yaml, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	var secret corev1.Secret
	_, _, err = s.Decode(yaml, nil, &secret)
	if err != nil {
		return nil, err
	}
	return &secret, nil
}

func TestTokenClaimCreateInterior(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("skupper", "", "")

	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "skupper",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	filename := "./conn1.yaml"
	err = cli.TokenClaimCreateFile(ctx, "link1", []byte("abcde"), 0, 5, filename)
	assert.Check(t, err, "Unable to create connector token")

	claim, err := readSecretFromFile(filename)
	assert.Check(t, err, "Unable to read claim")
	assert.Assert(t, len(claim.ObjectMeta.Annotations) > 0, "Claim has no annotations")
	assert.Assert(t, len(claim.ObjectMeta.Labels) > 0, "Claim has no labels")
	assert.Assert(t, len(claim.Data) > 0, "Claim has no data")
	assert.Equal(t, claim.ObjectMeta.Labels[types.SkupperTypeQualifier], types.TypeClaimRequest, "Claim does not have correct type indicator")
	urlstring, ok := claim.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey]
	u, err := url.Parse(urlstring)
	assert.Check(t, err, "Unable to parse claim url")
	assert.Assert(t, ok, "Claim has no url")
	assert.Assert(t, bytes.Equal(claim.Data[types.ClaimPasswordDataKey], []byte("abcde")), "Invalid password in claim")

	recordName := strings.Join(strings.Split(u.Path, "/"), "")
	record, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(recordName, metav1.GetOptions{})
	assert.Check(t, err, "Could not get claim record")
	assert.Assert(t, len(record.ObjectMeta.Annotations) > 0, "Claim record has no annotations")
	assert.Assert(t, len(record.ObjectMeta.Labels) > 0, "Claim record has no labels")
	assert.Assert(t, len(record.Data) > 0, "Claim record has no data")
	assert.Equal(t, record.ObjectMeta.Labels[types.SkupperTypeQualifier], types.TypeClaimRecord, "Claim record does not have correct type indicator")
	assert.Equal(t, record.ObjectMeta.Annotations[types.ClaimsRemaining], "5", "Invalid number of claims remaining")
	assert.Assert(t, bytes.Equal(record.Data[types.ClaimPasswordDataKey], []byte("abcde")), "Invalid password in claim record")

	os.Remove(filename)
	cli.KubeClient.CoreV1().Secrets(cli.Namespace).Delete(recordName, nil)
}

func TestTokenClaimCreateEdge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("skupper", "", "")

	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "skupper",
			RouterMode:        string(types.TransportModeEdge),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	err = cli.TokenClaimCreateFile(ctx, "conn1", []byte("abcde"), 0, 5, "./link1.yaml")
	assert.Error(t, err, "Edge configuration cannot accept connections", "Expect error when edge")

}
