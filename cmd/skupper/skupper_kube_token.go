package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

type SkupperKubeToken struct {
	kube *SkupperKube
}

func (s *SkupperKubeToken) GetCurrentSiteId(ctx context.Context) (string, error) {

	siteConfig, err := s.kube.Cli.SiteConfigInspect(ctx, nil)
	if err != nil || siteConfig == nil {

		return "", fmt.Errorf("Skupper is not enabled in namespace: %s", s.kube.Cli.GetNamespace())
	}

	return siteConfig.Reference.UID, nil
}

func (s *SkupperKubeToken) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeToken) Platform() types.Platform {
	return s.kube.Platform()
}

func (s *SkupperKubeToken) Create(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	if tokenTemplate != "" {
		return s.createFromTemplate(cmd, args)
	}
	cli := s.kube.Cli
	switch tokenType {
	case "cert":
		err := cli.ConnectorTokenCreateFile(context.Background(), clientIdentity, args[0])
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		return nil
	case "claim":
		name := clientIdentity
		if name == "skupper" {
			name = ""
		}
		if password == "" {
			password = utils.RandomId(24)
		}
		err := cli.TokenClaimCreateFile(context.Background(), name, []byte(password), expiry, uses, args[0])
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("invalid token type. Specify cert or claim")
	}
}

func (s *SkupperKubeToken) createFromTemplate(cmd *cobra.Command, args []string) error {
	cli := s.kube.Cli
	switch tokenType {
	case "cert":
		filename := args[0]
		secret, localOnly, err := cli.ConnectorTokenCreateFromTemplate(context.Background(), clientIdentity, tokenTemplate)
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		out, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("could not write to file %s:%s", filename, err.Error())
		}
		err = s.Encode(secret, out)
		if err != nil {
			return fmt.Errorf("could not write out generated secret: %s", err.Error())
		} else {
			var extra string
			if localOnly {
				extra = "(Note: token will only be valid for local cluster)"
			}
			fmt.Printf("Connection token written to %s %s", filename, extra)
			fmt.Println()
			return nil
		}
	case "claim":
		return fmt.Errorf("--template option cannot be used for a claim")
	default:
		return fmt.Errorf("invalid token type")
	}
}

func (s *SkupperKubeToken) Status(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	cli := s.kube.Cli.(*client.VanClient)

	if TokenStatusOpts.Type == types.TokenRoleClaim {
		secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(context.TODO(), TokenStatusOpts.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed getting kube secret: %s %w", TokenStatusOpts.Name, err)
		}

		fmt.Println("  namespace: ", cli.GetNamespace())
		if claims, ok := secret.ObjectMeta.Annotations[types.ClaimsMade]; ok {
			fmt.Println("  claims made: ", claims)
		} else {
			fmt.Println("  claims made: ", 0)
		}
		fmt.Println("  claims remaining: ", secret.ObjectMeta.Annotations[types.ClaimsRemaining])
		if expirationStr, ok := secret.ObjectMeta.Annotations[types.ClaimExpiration]; ok {
			expiration, timeErr := time.Parse(time.RFC3339, expirationStr)
			if timeErr == nil {
				if expiration.Before(time.Now()) {
					fmt.Printf("  Warning: token has expired, expiration time: %s\n", expiration)
				} else {
					fmt.Printf("  expires: %s\n", expiration)
				}
			}
		}
	} else {
		siteSecret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(context.TODO(), types.SiteServerSecret, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed getting kube secret: %s %w", types.SiteServerSecret, err)
		}

		// Check that the ca.crt from the token matches the ca.crt from skupper-site-sevice
		if strings.Compare(string(TokenStatusOpts.Secret.Data[types.ClaimCaCertDataKey]), string(siteSecret.Data[types.ClaimCaCertDataKey])) == 0 {
			fmt.Printf("  CA: Token matches %s\n", types.SiteServerSecret)
		} else {
			fmt.Println("  CA: does not match")
		}

		cert, err := certs.DecodeCertificate(siteSecret.Data["tls.crt"])
		if err != nil {
			return fmt.Errorf("failed to DecodeCertificate for %s %w", types.SiteServerSecret, err)
		}

		cert2, err := certs.DecodeCertificate(TokenStatusOpts.Secret.Data["tls.crt"])
		if err != nil {
			return fmt.Errorf("failed to DecodeCertificate for %s %w", TokenStatusOpts.Name, err)
		}

		// check that the hostname/IP is in the SANs list
		if secretHostname, ok := TokenStatusOpts.Secret.ObjectMeta.Annotations["inter-router-host"]; ok {
			hostFound := false
			for _, hostname := range cert.DNSNames {
				if secretHostname == hostname {
					hostFound = true
					fmt.Printf("  hostname: %s found in SANs list\n", secretHostname)
					break
				}
			}
			if !hostFound {
				fmt.Printf("  hostname: %s not found in SANs list\n", secretHostname)
			}
		}

		// check that skupper is the issuer of both secrets
		if cert.Issuer.CommonName == cert2.Issuer.CommonName {
			fmt.Printf("  Issuer: %s for both\n", cert.Issuer.CommonName)
		}

		fmt.Println()
		fmt.Println("Skupper-site-server Secret details:")
		fmt.Println("  AuthorityKeyId:", cert.AuthorityKeyId)
		fmt.Println("  DNS:", cert.DNSNames)
		fmt.Println("  IP:", cert.IPAddresses)
		fmt.Println("  Issuer:", cert.Issuer)
		fmt.Println("  Created:", siteSecret.CreationTimestamp)

		fmt.Println()
		fmt.Println("Token Secret details:")
		fmt.Println("  AuthorityKeyId:", cert2.AuthorityKeyId)
		fmt.Println("  Issuer:", cert2.Issuer)
	}

	return nil
}

func (s *SkupperKubeToken) CreateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&tokenType, "token-type", "t", "claim", "Type of token to create. Valid options are 'claim' or 'cert'")
	cmd.Flags().StringVarP(&password, "password", "p", "", "A password for the claim (only valid if --token-type=claim). If not specified one will be generated.")
	cmd.Flags().DurationVarP(&expiry, "expiry", "", 15*time.Minute, "Expiration time for claim (only valid if --token-type=claim)")
	cmd.Flags().IntVarP(&uses, "uses", "", 1, "Number of uses for which claim will be valid (only valid if --token-type=claim)")
	cmd.Flags().StringVarP(&tokenTemplate, "template", "", "", "The name of a secret used as a template for the token")
	f := cmd.Flag("template")
	f.Hidden = true
}
