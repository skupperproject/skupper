package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

type SkupperPodmanToken struct {
	podman      *SkupperPodman
	ingressHost string
}

func (s *SkupperPodmanToken) GetCurrentSiteId(ctx context.Context) (string, error) {

	if s.podman.currentSite == nil {
		return "", fmt.Errorf("Skupper is not enabled")
	}
	return s.podman.currentSite.Id, nil
}

func (s *SkupperPodmanToken) Create(cmd *cobra.Command, args []string) error {
	subject := clientIdentity
	secretFile := args[0]

	// Determining ingress host
	sitePodman := s.podman.currentSite
	if sitePodman.IsEdge() {
		return fmt.Errorf("Edge configuration cannot accept connections")
	}
	var defaultIngressHost string
	if len(sitePodman.IngressHosts) >= 2 {
		defaultIngressHost = sitePodman.IngressHosts[1]
	} else {
		return fmt.Errorf("tokens cannot be generated for sites initialized with ingress type none")
	}
	if s.ingressHost != "" {
		if !utils.StringSliceContains(sitePodman.IngressHosts, s.ingressHost) {
			return fmt.Errorf("tokens can only be generated for the available ingress hosts: %v", sitePodman.IngressHosts[1:])
		}
	}
	ingressHost := utils.DefaultStr(s.ingressHost, defaultIngressHost)
	if ingressHost == "" {
		return fmt.Errorf("Unable to determine ingress host (use --ingress-host)")
	}
	info := &domain.TokenCertInfo{
		InterRouterHost: ingressHost,
		InterRouterPort: strconv.Itoa(sitePodman.IngressBindInterRouterPort),
		EdgeHost:        ingressHost,
		EdgePort:        strconv.Itoa(sitePodman.IngressBindEdgePort),
	}

	// Retrieving CA
	credHandler := podman.NewPodmanCredentialHandler(s.podman.cli)

	// Creating secret
	tokenHandler := &podman.TokenCertHandler{}
	return tokenHandler.Create(secretFile, subject, info, sitePodman, credHandler)
}

func (s *SkupperPodmanToken) Status(cmd *cobra.Command, args []string) error {
	// Retrieving CA
	credHandler := podman.NewPodmanCredentialHandler(s.podman.cli)
	ca, err := credHandler.GetSecret(types.SiteServerSecret)
	if err != nil {
		return fmt.Errorf("unable to find ca %s - %w", types.ServiceCaSecret, err)
	}

	// Check that the ca.crt from the token matches the ca.crt from skupper-site-sevice
	if strings.Compare(string(TokenStatusOpts.Secret.Data[types.ClaimCaCertDataKey]), string(ca.Data["ca.crt"])) == 0 {
		fmt.Printf("  CA: Token matches %s\n", types.SiteServerSecret)
	} else {
		fmt.Println("  CA: does not match")
	}

	cert, err := certs.DecodeCertificate(ca.Data["tls.crt"])
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

	fmt.Println()
	fmt.Println("Token Secret details:")
	fmt.Println("  AuthorityKeyId:", cert2.AuthorityKeyId)
	fmt.Println("  Issuer:", cert2.Issuer)

	return nil
}

func (s *SkupperPodmanToken) CreateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&s.ingressHost, "ingress-host", "", "", "Hostname or alias by which the ingress route or proxy can be reached")
}

func (s *SkupperPodmanToken) NewClient(cmd *cobra.Command, args []string) {
	s.podman.NewClient(cmd, args)
}

func (s *SkupperPodmanToken) Platform() types.Platform {
	return s.podman.Platform()
}
