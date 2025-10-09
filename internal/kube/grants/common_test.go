package grants

import (
	"errors"
	"io"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type FailingReader struct {
	reader io.Reader
	read   int
	fail   int
}

func (r *FailingReader) Read(p []byte) (int, error) {
	if r.reader == nil {
		return 0, errors.New("Failed Read")
	}
	n, err := r.reader.Read(p)
	if r.read+n > r.fail {
		return 0, errors.New("Failed Read")
	}
	r.read += n
	return n, err
}

type FailingWriter struct {
	writer io.Writer
	calls  int
	fail   int
}

func (r *FailingWriter) Write(p []byte) (int, error) {
	if r.writer == nil {
		return 0, errors.New("Failed Write")
	}
	n, err := r.writer.Write(p)
	r.calls = r.calls + 1
	if r.calls > r.fail {
		return 0, errors.New("Failed Write")
	}
	return n, err
}

func (*factory) pod(name string, namespace string, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
	}
}

func (*factory) cert(name string, namespace string, subject string, ca string, signing bool, client bool, server bool, ownerRefs []metav1.OwnerReference) *v2alpha1.Certificate {
	return &v2alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: ownerRefs,
		},
		Spec: v2alpha1.CertificateSpec{
			Ca:      ca,
			Subject: subject,
			Signing: signing,
			Client:  client,
			Server:  server,
		},
	}
}

func (*factory) securedAccess(name string, namespace string, selector map[string]string, port int, ownerRefs []metav1.OwnerReference) *v2alpha1.SecuredAccess {
	return &v2alpha1.SecuredAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "SecuredAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: ownerRefs,
		},
		Spec: v2alpha1.SecuredAccessSpec{
			Selector: selector,
			Ports: []v2alpha1.SecuredAccessPort{
				{
					Name: "https",
					Port: port,
				},
			},
			Issuer:      "skupper-grant-server-ca",
			Certificate: "skupper-grant-server",
		},
	}
}

func (*factory) grant(name string, namespace string, uid string) *v2alpha1.AccessGrant {
	if uid == "" {
		uid = uuid.NewString()
	}
	return &v2alpha1.AccessGrant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AccessGrant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Spec: v2alpha1.AccessGrantSpec{
			RedemptionsAllowed: 1,
		},
	}

}

func (*factory) secret(name string, namespace string, subject string, hosts []string) (*corev1.Secret, error) {
	secret, err := certs.GenerateSecret(name, subject, hosts, 0, nil)
	if err != nil {
		return nil, err
	}
	secret.ObjectMeta.Namespace = namespace
	return secret, nil
}

func (*factory) genericSecret(name string, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uuid.NewString()),
		},
	}
}

func (*factory) site(name string, namespace string) *v2alpha1.Site {
	return &v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uuid.NewString()),
		},
	}
}

func (*factory) link(name string, namespace string, endpoints []v2alpha1.Endpoint, tlsCredentials string) *v2alpha1.Link {
	return &v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uuid.NewString()),
		},
		Spec: v2alpha1.LinkSpec{
			Endpoints:      endpoints,
			TlsCredentials: tlsCredentials,
		},
	}
}

func (*factory) token(name string, namespace string, url string, code string, ca string) *v2alpha1.AccessToken {
	return &v2alpha1.AccessToken{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AccessToken",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v2alpha1.AccessTokenSpec{
			Url:  url,
			Code: code,
			Ca:   ca,
		},
	}
}

func (*factory) addLinkCost(token *v2alpha1.AccessToken, linkCost int) *v2alpha1.AccessToken {
	token.Spec.LinkCost = linkCost
	return token
}

func (*factory) addCost(link *v2alpha1.Link, cost int) *v2alpha1.Link {
	link.Spec.Cost = cost
	return link
}

type factory struct{}

var tf = &factory{}
