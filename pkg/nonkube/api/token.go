package api

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"slices"
	"strconv"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/utils"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

type Token struct {
	Links  []*v2alpha1.Link
	Secret *v1.Secret
}

func (t *Token) Marshal() ([]byte, error) {
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	buffer := new(bytes.Buffer)
	writer := bufio.NewWriter(buffer)
	_, _ = writer.Write([]byte("---\n"))
	err := s.Encode(t.Secret, writer)
	if err != nil {
		return nil, err
	}
	for _, l := range t.Links {
		_, _ = writer.Write([]byte("---\n"))
		err = s.Encode(l, writer)
		if err != nil {
			return nil, err
		}
		if err = writer.Flush(); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

func CreateTokens(routerAccess v2alpha1.RouterAccess, serverSecret v1.Secret, clientSecret v1.Secret) []*Token {
	var tokens []*Token
	interRouter := 0
	edge := 0
	for _, role := range routerAccess.Spec.Roles {
		switch role.Name {
		case "inter-router":
			interRouter = role.Port
		case "edge":
			edge = role.Port
		}
	}
	if interRouter == 0 && edge == 0 {
		return nil
	}
	createToken := func(host string) *Token {
		name := routerAccess.Name
		linkName := fmt.Sprintf("link-%s", name)
		// adjusting name to match the standard used by pkg/site/link.go
		clientSecret.Name = fmt.Sprintf("link-%s", name)
		token := &Token{
			Links: []*v2alpha1.Link{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "Link",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: linkName,
					},
					Spec: v2alpha1.LinkSpec{
						TlsCredentials: clientSecret.Name,
						Cost:           1,
					},
				},
			},
			Secret: &clientSecret,
		}
		var endpoints []v2alpha1.Endpoint
		if interRouter > 0 {
			endpoints = append(endpoints, v2alpha1.Endpoint{
				Name:  "inter-router",
				Host:  host,
				Port:  strconv.Itoa(interRouter),
				Group: "", // TODO ?
			})
		}
		if edge > 0 {
			endpoints = append(endpoints, v2alpha1.Endpoint{
				Name:  "edge",
				Host:  host,
				Port:  strconv.Itoa(edge),
				Group: "", // TODO ?
			})
		}
		token.Links[0].Spec.Endpoints = endpoints
		token.Secret.Namespace = ""
		return token
	}
	var hosts []string
	hosts = append(hosts, utils.DefaultStr(routerAccess.Spec.BindHost, "127.0.0.1"))
	// reading SANs from server certificate
	serverCertificateData := serverSecret.Data["tls.crt"]
	serverCertificateBlk, _ := pem.Decode(serverCertificateData)
	if serverCertificateBlk != nil {
		serverCertificate, err := x509.ParseCertificate(serverCertificateBlk.Bytes)
		if err == nil {
			for _, ipAddr := range serverCertificate.IPAddresses {
				if ipAddr.String() != "" && !slices.Contains(hosts, ipAddr.String()) {
					hosts = append(hosts, ipAddr.String())
				}
			}
			for _, dnsName := range serverCertificate.DNSNames {
				if dnsName != "" && !slices.Contains(hosts, dnsName) {
					hosts = append(hosts, dnsName)
				}
			}
		}
	}
	// if no server certificate provided, use routerAccess.spec.subjectAlternativeNames
	if len(hosts) == 1 {
		if len(routerAccess.Spec.SubjectAlternativeNames) > 0 {
			hosts = append(hosts, routerAccess.Spec.SubjectAlternativeNames...)
		}
	}
	for _, host := range hosts {
		tokens = append(tokens, createToken(host))
	}
	return tokens
}
