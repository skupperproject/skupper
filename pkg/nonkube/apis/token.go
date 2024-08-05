package apis

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

type Token struct {
	Links  []*v1alpha1.Link
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

func CreateToken(routerAccess *v1alpha1.RouterAccess, secret *v1.Secret) *Token {
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
	name := routerAccess.Name
	linkName := fmt.Sprintf("link-%s", name)
	// adjusting name to match the standard used by pkg/site/link.go
	secret.Name = fmt.Sprintf("link-%s", name)
	token := &Token{
		Links: []*v1alpha1.Link{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v1alpha1",
					Kind:       "Link",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: linkName,
				},
				Spec: v1alpha1.LinkSpec{
					TlsCredentials: secret.Name,
					Cost:           1,
				},
			},
		},
		Secret: secret,
	}
	linkHost := utils.DefaultStr(routerAccess.Spec.BindHost, "127.0.0.1")
	var endpoints []v1alpha1.Endpoint
	if interRouter > 0 {
		endpoints = append(endpoints, v1alpha1.Endpoint{
			Name:  "inter-router",
			Host:  linkHost,
			Port:  strconv.Itoa(interRouter),
			Group: "", // TODO ?
		})
	}
	if edge > 0 {
		endpoints = append(endpoints, v1alpha1.Endpoint{
			Name:  "edge",
			Host:  linkHost,
			Port:  strconv.Itoa(edge),
			Group: "", // TODO ?
		})
	}
	token.Links[0].Spec.Endpoints = endpoints
	return token
}
