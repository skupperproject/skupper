package claims

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
)

type ClaimOptions struct {
	Name     string
	Password []byte
	Expiry   time.Duration
	Uses     int
}

type SiteContext interface {
	resolver.Resolver
	IsEdge() bool
	GetSiteVersion() string
	GetSiteId() string
	GetOwnerReferences() []metav1.OwnerReference
}

type ClaimFactory struct {
	clients     kube.Clients
	namespace   string
	ctx         context.Context
	siteContext SiteContext
}

func (o *ClaimOptions) checkName() error {
	if o.Name == "" {
		id, err := uuid.NewUUID()
		if err != nil {
			return err
		}
		o.Name = id.String()
	}
	return nil
}

func checkOptions(name string, password []byte, expiry time.Duration, uses int) (*ClaimOptions, error) {
	options := &ClaimOptions{
		Name:     name,
		Password: password,
		Expiry:   expiry,
		Uses:     uses,
	}
	err := options.checkName()
	if err != nil {
		return nil, err
	}
	return options, nil
}

func NewClaimFactory(clients kube.Clients, namespace string, siteContext SiteContext, ctx context.Context) *ClaimFactory {
	return &ClaimFactory{
		clients:     clients,
		namespace:   namespace,
		ctx:         ctx,
		siteContext: siteContext,
	}
}

func (m *ClaimFactory) CreateTokenClaim(name string, password []byte, expiry time.Duration, uses int) (*corev1.Secret, error) {
	var expiryStr string
	options, err := checkOptions(name, password, expiry, uses)
	if err != nil {
		return nil, err
	}

	if m.siteContext.IsEdge() {
		return nil, fmt.Errorf("Edge configuration cannot accept connections")
	}

	if expiry > 0 {
		expiration := time.Now().Add(expiry)
		expiryStr = expiration.Format(time.RFC3339)
	}
	claim, err := m.createClaimToken(options.Name, options.Password, expiryStr)
	if err != nil {
		return nil, err
	}
	err = m.createClaimRecord(options.Name, options.Password, expiryStr, options.Uses)
	if err != nil {
		return nil, err
	}

	return claim, nil
}

func (m *ClaimFactory) RecreateTokenClaim(name string) (*corev1.Secret, error) {
	var expiryStr string
	secret, err := m.clients.GetKubeClient().CoreV1().Secrets(m.namespace).Get(m.ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if !isTokenRecord(secret) {
		return nil, nil
	}
	password := secret.Data[types.ClaimPasswordDataKey]
	if secret.ObjectMeta.Annotations[types.ClaimExpiration] != "" {
		expiryStr = secret.ObjectMeta.Annotations[types.ClaimExpiration]
	}
	token, err := m.createClaimToken(name, password, expiryStr)
	return token, err
}

func (m *ClaimFactory) createClaimRecord(name string, password []byte, expiry string, uses int) error {
	record := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRecord,
			},
			Annotations: map[string]string{
				types.SiteVersion: m.siteContext.GetSiteVersion(),
			},
		},
		Data: map[string][]byte{
			types.ClaimPasswordDataKey: password,
		},
	}
	record.ObjectMeta.OwnerReferences = m.siteContext.GetOwnerReferences()
	if expiry != "" {
		record.ObjectMeta.Annotations[types.ClaimExpiration] = expiry
	}
	if uses > 0 {
		record.ObjectMeta.Annotations[types.ClaimsRemaining] = strconv.Itoa(uses)
	}
	_, err := m.clients.GetKubeClient().CoreV1().Secrets(m.namespace).Create(m.ctx, &record, metav1.CreateOptions{})
	return err
}

func (m *ClaimFactory) createClaimToken(name string, password []byte, expiry string) (*corev1.Secret, error) {
	hostPort, err := m.siteContext.GetHostPortForClaims()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://%s:%d/%s", hostPort.Host, hostPort.Port, name)

	caSecret, err := m.clients.GetKubeClient().CoreV1().Secrets(m.namespace).Get(m.ctx, types.SiteCaSecret, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	claim := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRequest,
			},
			Annotations: map[string]string{
				types.ClaimUrlAnnotationKey: url,
				types.SiteVersion:           m.siteContext.GetSiteVersion(),
				types.TokenGeneratedBy:      m.siteContext.GetSiteId(),
			},
		},
		Data: map[string][]byte{
			types.ClaimPasswordDataKey: password,
			types.ClaimCaCertDataKey:   caSecret.Data["tls.crt"],
		},
	}
	claim.ObjectMeta.OwnerReferences = m.siteContext.GetOwnerReferences()
	if expiry != "" {
		claim.ObjectMeta.Annotations[types.ClaimExpiration] = expiry
	}

	return &claim, nil
}

// TODO: this is duplicated from cmd/service-controller/tokens.go
func isTokenRecord(s *corev1.Secret) bool {
	if s.ObjectMeta.Labels != nil {
		if typename, ok := s.ObjectMeta.Labels[types.SkupperTypeQualifier]; ok {
			return typename == types.TypeClaimRecord
		}
	}
	return false
}
