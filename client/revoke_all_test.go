package client

import (
	"bytes"
	"context"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"

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
}
