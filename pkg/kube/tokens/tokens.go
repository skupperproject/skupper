package tokens

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
)

type CertToken struct {
	linkConfig     *skupperv1alpha1.Link
	tlsCredentials *corev1.Secret
}

type ClaimToken struct {
}

type Token interface {
	Write(writer io.Writer) error
}

type TokenGenerator struct {
	namespace   string
	clients     kube.Clients
	ca          *corev1.Secret
	interRouter skupperv1alpha1.HostPort
	edge        skupperv1alpha1.HostPort
	claims      skupperv1alpha1.HostPort
	hosts       []string
}

func NewTokenGenerator(namespace string, clients kube.Clients) (*TokenGenerator, error) {
	generator := &TokenGenerator{
		namespace: namespace,
		clients:   clients,
	}
	if err := generator.loadCA(); err != nil {
		return nil, err
	}
	if err := generator.loadValidHosts(); err != nil {
		return nil, err
	}
	return generator, nil
}

func NewTokenGeneratorForSite(site *skupperv1alpha1.Site, clients kube.Clients) (*TokenGenerator, error) {
	generator := &TokenGenerator{
		namespace: site.Namespace,
		clients:   clients,
	}
	if err := generator.loadCA(); err != nil {
		return nil, err
	}
	if err := generator.setValidHostsFromSite(site); err != nil {
		return nil, err
	}
	return generator, nil
}

func (g *TokenGenerator) loadCA() error {
	ca, err := g.clients.GetKubeClient().CoreV1().Secrets(g.namespace).Get(context.TODO(), "skupper-site-ca", metav1.GetOptions{})
	if err != nil {
		return err
	}
	g.ca = ca
	return nil
}

func (g *TokenGenerator) getActiveSite() (*skupperv1alpha1.Site, error) {
	sites, err := g.clients.GetSkupperClient().SkupperV1alpha1().Sites(g.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, site := range sites.Items {
		if site.Status.Active {
			return &site, nil
		}
	}
	return nil, fmt.Errorf("No active site in %s", g.namespace)
}

func setHostPortFromEndpoint(o *skupperv1alpha1.HostPort, endpoint *skupperv1alpha1.Endpoint) {
	o.Host = endpoint.Host
	o.Port, _ = strconv.Atoi(endpoint.Port) //TODO: why different types here?
}

func (g *TokenGenerator) getHostPortByName(name string) *skupperv1alpha1.HostPort {
	if name == "inter-router" {
		return &g.interRouter
	} else if name == "edge" {
		return &g.edge
	} else if name == "claims" {
		return &g.claims
	}
	return nil
}

func (g *TokenGenerator) loadValidHosts() error {
	site, err := g.getActiveSite()
	if err != nil {
		return err
	}
	return g.setValidHostsFromSite(site)
}

func (g *TokenGenerator) setValidHostsFromSite(site *skupperv1alpha1.Site) error {
	//TODO: if site is edge site, then return an error as it
	//cannot issue certificates
	var hosts []string
	for _, endpoint := range site.Status.Endpoints {
		if hp := g.getHostPortByName(endpoint.Name); hp != nil && endpoint.Host != "" {
			setHostPortFromEndpoint(hp, &endpoint)
			hosts = append(hosts, endpoint.Host)
		}
	}
	if len(hosts) == 0 {
		return fmt.Errorf("Endpoints for site in %s not yet resolved", g.namespace)
	}
	g.hosts = hosts
	return nil
}

func (g *TokenGenerator) NewCertToken(name string, subject string) Token {
	cert := certs.GenerateSecret(name, subject, strings.Join(g.hosts, ","), g.ca)
	return &CertToken{
		linkConfig: &skupperv1alpha1.Link{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
				Kind:       "Link",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: skupperv1alpha1.LinkSpec{
				InterRouter:    g.interRouter,
				Edge:           g.edge,
				TlsCredentials: name,
			},
		},
		tlsCredentials: &cert,
	}
}

func (t *CertToken) Write(writer io.Writer) error {
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	writer.Write([]byte("---\n"))
	err := s.Encode(t.linkConfig, writer)
	if err != nil {
		return err
	}
	writer.Write([]byte("---\n"))
	err = s.Encode(t.tlsCredentials, writer)
	if err != nil {
		return err
	}
	return nil
}
