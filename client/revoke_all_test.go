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

	ca1, _, err := cli.SecretManager(cli.Namespace).GetSecret(types.SiteCaSecret)
	assert.Check(t, err, "Unable to get CA before revocation")

	cert1, _, err := cli.SecretManager(cli.Namespace).GetSecret(types.SiteServerSecret)
	assert.Check(t, err, "Unable to get cert before revocation")

	cli.RevokeAccess(ctx)

	ca2, _, err := cli.SecretManager(cli.Namespace).GetSecret(types.SiteCaSecret)
	assert.Check(t, err, "Unable to get CA before revocation")

	cert2, _, err := cli.SecretManager(cli.Namespace).GetSecret(types.SiteServerSecret)
	assert.Check(t, err, "Unable to get cert before revocation")

	for key, value := range ca1.Data {
		assert.Assert(t, !bytes.Equal(ca2.Data[key], value), "Same value for "+key)
	}
	for key, value := range cert1.Data {
		assert.Assert(t, !bytes.Equal(cert2.Data[key], value), "Same value for "+key)
	}
	_, _, err = cli.SecretManager(cli.Namespace).GetSecret(recordName)
	assert.Assert(t, err != nil, "Expected error when retrieving claim record")
	assert.Assert(t, errors.IsNotFound(err), "claim record still exists")
}
