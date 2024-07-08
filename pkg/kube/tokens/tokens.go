package tokens

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/certs"
	sitepkg "github.com/skupperproject/skupper/pkg/site"
)

type CertToken struct {
	links          []*skupperv1alpha1.Link
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
	endpoints [][]skupperv1alpha1.Endpoint
	hosts     []string
}

func NewTokenGenerator(namespace string, clients internalclient.Clients) (*TokenGenerator, error) {
	site, err := getActiveSite(namespace, clients)
	if err != nil {
		return nil, err
	}
	return NewTokenGeneratorForSite(site, clients)
}

func NewTokenGeneratorForSite(site *skupperv1alpha1.Site, clients internalclient.Clients) (*TokenGenerator, error) {
	generator := &TokenGenerator{
		namespace: site.Namespace,
		clients:   clients,
	}
	if err := generator.loadCA(sitepkg.DefaultIssuer(site)); err != nil {
		return nil, err
	}
	if err := generator.setValidHostsFromSite(site); err != nil {
		return nil, err
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

func getActiveSite(namespace string, clients internalclient.Clients) (*skupperv1alpha1.Site, error) {
	sites, err := clients.GetSkupperClient().SkupperV1alpha1().Sites(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, site := range sites.Items {
		if site.Status.Active {
			return &site, nil
		}
	}
	return nil, fmt.Errorf("No active site in %s", namespace)
}

func (g *TokenGenerator) setValidHostsFromSite(site *skupperv1alpha1.Site) error {
	//TODO: if site is edge site, then return an error as it
	//cannot issue certificates
	var hosts []string
	for _, endpoint := range site.Status.Endpoints {
		hosts = append(hosts, endpoint.Host)
	}
	if len(hosts) == 0 {
		return fmt.Errorf("Endpoints for site in %s not yet resolved", g.namespace)
	}
	byGroup := map[string][]skupperv1alpha1.Endpoint{}
	//TODO: should only include groups that are valid for the defined issuer
	for _, endpoint := range site.Status.Endpoints {
		if endpoint.Name == "inter-router" || endpoint.Name == "edge" {
			byGroup[endpoint.Group] = append(byGroup[endpoint.Group], endpoint)
		}
	}
	for _, endpoints := range byGroup {
		g.endpoints = append(g.endpoints, endpoints)
	}
	log.Printf("Endpoints for grant: %v (by group: %v)", g.endpoints, byGroup)
	g.hosts = hosts
	return nil
}

func (g *TokenGenerator) NewCertToken(name string, subject string) Token {
	cert := certs.GenerateSecret(name, subject, strings.Join(g.hosts, ","), g.ca)
	token := &CertToken{
		tlsCredentials: &cert,
	}
	for i, endpoints := range g.endpoints {
		linkName := name
		if len(g.endpoints) > 1 {
			linkName = fmt.Sprintf("%s-%d", name, i+1)
		}
		link := &skupperv1alpha1.Link{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
				Kind:       "Link",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: linkName,
			},
			Spec: skupperv1alpha1.LinkSpec{
				Endpoints:      endpoints,
				TlsCredentials: name,
			},
		}
		token.links = append(token.links, link)
	}
	return token
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
