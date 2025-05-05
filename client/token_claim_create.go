package client

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/skupperproject/skupper/pkg/kube/claims"
	"github.com/skupperproject/skupper/pkg/kube/site"
)

func getSiteId(service *corev1.Service) string {
	for _, ref := range service.ObjectMeta.OwnerReferences {
		if ref.Name == "skupper-site" {
			return string(ref.UID)
		}
	}
	return ""
}

func (cli *VanClient) TokenClaimCreateFile(ctx context.Context, name string, password []byte, expiry time.Duration, uses int, secretFile string) error {
	policy := NewPolicyValidatorAPI(cli)
	res, err := policy.IncomingLink()
	if err != nil {
		return err
	}
	if !res.Allowed {
		return res.Err()
	}
	claim, localOnly, err := cli.TokenClaimCreate(ctx, name, password, expiry, uses)
	if err != nil {
		return err
	}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	out, err := os.Create(secretFile)
	if err != nil {
		return fmt.Errorf("could not write to file %s:%s", secretFile, err.Error())
	}
	err = s.Encode(claim, out)
	if err != nil {
		return fmt.Errorf("could not write out generated secret: %s", err.Error())
	} else {
		var extra string
		if localOnly {
			extra = "(Note: token will only be valid for local cluster)"
		}
		fmt.Printf("Token written to %s %s", secretFile, extra)
		fmt.Println()
		return nil
	}
}

func (cli *VanClient) TokenClaimCreate(ctx context.Context, name string, password []byte, expiry time.Duration, uses int) (*corev1.Secret, bool, error) {
	policy := NewClusterPolicyValidator(cli)
	res := policy.ValidateIncomingLink()
	if !res.Allowed() {
		return nil, false, fmt.Errorf("incoming links are not allowed")
	}

	siteContext, err := site.GetSiteContext(cli, cli.Namespace, ctx)
	if err != nil {
		return nil, false, err
	}

	token, err := claims.NewClaimFactory(cli, cli.Namespace, siteContext, ctx).CreateTokenClaim(name, password, expiry, uses)
	if err != nil {
		return nil, false, err
	}
	return token, siteContext.IsLocalAccessOnly(), nil
}
