package grants

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/skupperproject/skupper/internal/certs"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type CertToken struct {
	links          []*skupperv2alpha1.Link
	tlsCredentials *corev1.Secret
}

type ClaimToken struct {
}

type Token interface {
	Write(writer io.Writer) error
}

type TokenGenerator struct {
	namespace string
	clients   internalclient.Clients
	ca        *corev1.Secret
	endpoints [][]skupperv2alpha1.Endpoint
	hosts     []string
	logger    *slog.Logger
}

func NewTokenGenerator(site *skupperv2alpha1.Site, clients internalclient.Clients) (*TokenGenerator, error) {
	generator := &TokenGenerator{
		namespace: site.Namespace,
		clients:   clients,
		logger:    slog.New(slog.Default().Handler()).With(slog.String("component", "kube.grants.tokenGenerator")),
	}
	if err := generator.loadCA(site.DefaultIssuer()); err != nil {
		generator.logger.Error("Error retrieving default issuer for site",
			slog.String("defaultIssuer", site.DefaultIssuer()),
			slog.String("namespace", site.Namespace),
			slog.String("name", site.Name),
			slog.Any("error", err))
		return nil, errors.New("Could not get issuer for requested certificate")
	}
	if ok := generator.setValidHostsFromSite(site); !ok {
		generator.logger.Error("Could not resolve any target endpoints for site", slog.String("namespace", site.Namespace), slog.String("name", site.Name))
		return nil, errors.New("Could not resolve any endpoints for requested link")
	}
	return generator, nil
}

func (g *TokenGenerator) loadCA(name string) error {
	ca, err := g.clients.GetKubeClient().CoreV1().Secrets(g.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	g.ca = ca
	return nil
}

func (g *TokenGenerator) setValidHostsFromSite(site *skupperv2alpha1.Site) bool {
	//TODO: if site is edge site, then return an error as it
	//cannot issue certificates
	var hosts []string
	for _, endpoint := range site.Status.Endpoints {
		hosts = append(hosts, endpoint.Host)
	}
	if len(hosts) == 0 {
		return false
	}
	byGroup := map[string][]skupperv2alpha1.Endpoint{}
	//TODO: should only include groups that are valid for the defined issuer
	for _, endpoint := range site.Status.Endpoints {
		if endpoint.Name == "inter-router" || endpoint.Name == "edge" {
			byGroup[endpoint.Group] = append(byGroup[endpoint.Group], endpoint)
		}
	}
	for _, endpoints := range byGroup {
		g.endpoints = append(g.endpoints, endpoints)
	}
	g.logger.Info("Endpoints for grant by group", slog.Any("endpoints", g.endpoints), slog.Any("byGroup", byGroup))
	g.hosts = hosts
	return true
}

func (g *TokenGenerator) NewCertToken(name string, subject string) (Token, error) {
	cert, err := certs.GenerateSecret(name, subject, g.hosts, 0, g.ca)
	if err != nil {
		return nil, err
	}
	token := &CertToken{
		tlsCredentials: cert,
	}
	for i, endpoints := range g.endpoints {
		linkName := name
		if len(g.endpoints) > 1 {
			linkName = fmt.Sprintf("%s-%d", name, i+1)
		}
		link := &skupperv2alpha1.Link{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v2alpha1",
				Kind:       "Link",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: linkName,
			},
			Spec: skupperv2alpha1.LinkSpec{
				Endpoints:      endpoints,
				TlsCredentials: name,
			},
		}
		token.links = append(token.links, link)
	}
	return token, nil
}

func (t *CertToken) Write(writer io.Writer) error {
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	writer.Write([]byte("---\n"))
	err := s.Encode(t.tlsCredentials, writer)
	if err != nil {
		return err
	}
	for _, link := range t.links {
		writer.Write([]byte("---\n"))
		err = s.Encode(link, writer)
		if err != nil {
			return err
		}
	}
	return nil
}
