package client

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRevokeAccess(t *testing.T) {
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
	assert.Check(t, err, "Unable to create router")

	filename := "./link1.yaml"
	err = cli.TokenClaimCreateFile(ctx, "link1", []byte("abcde"), 0, 5, filename)
	assert.Check(t, err, "Unable to create claim")
	claim, err := readSecretFromFile(filename)
	assert.Check(t, err, "Unable to read claim")
	urlstring, ok := claim.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey]
	assert.Assert(t, ok, "Claim has no url")
	u, err := url.Parse(urlstring)
	assert.Check(t, err, "Unable to parse claim url")
	recordName := strings.Join(strings.Split(u.Path, "/"), "")
	os.Remove(filename)

	ca1, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.SiteCaSecret, metav1.GetOptions{})
	assert.Check(t, err, "Unable to get CA before revocation")

	cert1, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.SiteServerSecret, metav1.GetOptions{})
	assert.Check(t, err, "Unable to get cert before revocation")

	cli.RevokeAccess(ctx)

	ca2, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.SiteCaSecret, metav1.GetOptions{})
	assert.Check(t, err, "Unable to get CA before revocation")

	cert2, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.SiteServerSecret, metav1.GetOptions{})
	assert.Check(t, err, "Unable to get cert before revocation")

	for key, value := range ca1.Data {
		assert.Assert(t, !bytes.Equal(ca2.Data[key], value), "Same value for "+key)
	}
	for key, value := range cert1.Data {
		assert.Assert(t, !bytes.Equal(cert2.Data[key], value), "Same value for "+key)
	}
	_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(recordName, metav1.GetOptions{})
	assert.Assert(t, err != nil, "Expected error when retrieving claim record")
	assert.Assert(t, errors.IsNotFound(err), "claim record still exists")
}
