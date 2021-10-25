package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gorilla/mux"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/utils"
)

const (
	TokenManagement string = "TokenManagement"
)

type TokenState struct {
	Name            string     `json:"name"`
	ClaimsMade      *int       `json:"claimsMade"`
	ClaimsRemaining *int       `json:"claimsRemaining"`
	ClaimExpiration *time.Time `json:"claimExpiration"`
	Created         string     `json:"created,omitempty"`
}

func getIntAnnotation(key string, s *corev1.Secret) *int {
	if value, ok := s.ObjectMeta.Annotations[key]; ok {
		result, err := strconv.Atoi(value)
		if err == nil {
			return &result
		}
	}
	return nil
}

func getClaimsRemaining(s *corev1.Secret) *int {
	return getIntAnnotation(types.ClaimsRemaining, s)
}

func getClaimsMade(s *corev1.Secret) *int {
	return getIntAnnotation(types.ClaimsMade, s)
}

func getClaimExpiration(s *corev1.Secret) *time.Time {
	if value, ok := s.ObjectMeta.Annotations[types.ClaimExpiration]; ok {
		result, err := time.Parse(time.RFC3339, value)
		if err == nil {
			return &result
		}
	}
	return nil
}

func isTokenRecord(s *corev1.Secret) bool {
	if s.ObjectMeta.Labels != nil {
		if typename, ok := s.ObjectMeta.Labels[types.SkupperTypeQualifier]; ok {
			return typename == types.TypeClaimRecord
		}
	}
	return false
}

func getTokenState(s *corev1.Secret) TokenState {
	return TokenState{
		Name:            s.ObjectMeta.Name,
		ClaimsRemaining: getClaimsRemaining(s),
		ClaimsMade:      getClaimsMade(s),
		ClaimExpiration: getClaimExpiration(s),
		Created:         s.ObjectMeta.CreationTimestamp.Format(time.RFC3339),
	}
}

type TokenManager struct {
	cli *client.VanClient
}

func newTokenManager(cli *client.VanClient) *TokenManager {
	return &TokenManager{
		cli: cli,
	}
}

func (m *TokenManager) getTokens() ([]TokenState, error) {
	tokens := []TokenState{}
	secrets, err := m.cli.KubeClient.CoreV1().Secrets(m.cli.Namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=token-claim-record"})
	if err != nil {
		return tokens, err
	}
	for _, s := range secrets.Items {
		tokens = append(tokens, getTokenState(&s))
	}
	return tokens, nil
}

func (m *TokenManager) getToken(name string) (*TokenState, error) {
	secret, err := m.cli.KubeClient.CoreV1().Secrets(m.cli.Namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if !isTokenRecord(secret) {
		return nil, nil
	}
	token := getTokenState(secret)
	return &token, nil
}

func (m *TokenManager) deleteToken(name string) (bool, error) {
	secret, err := m.cli.KubeClient.CoreV1().Secrets(m.cli.Namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	} else if !isTokenRecord(secret) {
		return false, nil
	}
	err = m.cli.KubeClient.CoreV1().Secrets(m.cli.Namespace).Delete(name, &metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	event.Recordf(TokenManagement, "Deleted token %q", name)
	return true, nil
}

type TokenOptions struct {
	Expiry time.Duration
	Uses   int
}

func (m *TokenManager) generateToken(options *TokenOptions) (*corev1.Secret, error) {
	password := utils.RandomId(128)
	claim, _, err := m.cli.TokenClaimCreate(context.Background(), "", []byte(password), options.Expiry, options.Uses)
	if err != nil {
		return nil, err
	}
	return claim, nil
}

func (m *TokenManager) downloadClaim(name string) (*corev1.Secret, error) {
	secret, err := m.cli.KubeClient.CoreV1().Secrets(m.cli.Namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if !isTokenRecord(secret) {
		return nil, nil
	}
	password := secret.Data[types.ClaimPasswordDataKey]
	claim, _, _, err := m.cli.TokenClaimTemplateCreate(context.Background(), name, password, name)
	return claim, err
}

func (o *TokenOptions) setExpiration(value string) error {
	if value == "" {
		o.Expiry = 15 * time.Minute
		return nil
	}
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	o.Expiry = result.Sub(time.Now())
	return nil
}

func (o *TokenOptions) setUses(value string) error {
	if value == "" {
		o.Uses = 1
		return nil
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	o.Uses = result
	return nil
}

func getTokenOptions(r *http.Request) (*TokenOptions, error) {
	options := &TokenOptions{}
	params := r.URL.Query()
	err := options.setExpiration(params.Get("expiration"))
	if err != nil {
		return nil, err
	}
	err = options.setUses(params.Get("uses"))
	if err != nil {
		return nil, err
	}
	return options, nil
}

func serveTokens(m *TokenManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if r.Method == http.MethodGet {
			if name, ok := vars["name"]; ok {
				token, err := m.getToken(name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else if token == nil {
					http.Error(w, "No such token", http.StatusNotFound)
				} else {
					writeJson(token, w)
				}

			} else {
				tokens, err := m.getTokens()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else {
					writeJson(tokens, w)
				}
			}
		} else if r.Method == http.MethodPost {
			options, err := getTokenOptions(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				token, err := m.generateToken(options)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else {
					writeJson(token, w)
				}
			}
		} else if r.Method == http.MethodDelete {
			if name, ok := vars["name"]; ok {
				ok, err := m.deleteToken(name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else if !ok {
					http.Error(w, "No such token", http.StatusNotFound)
				} else {
					event.Recordf("Token %s deleted", name)
				}
			} else {
				http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			}
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})
}

func downloadClaim(m *TokenManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if r.Method == http.MethodGet {
			if name, ok := vars["name"]; ok {
				token, err := m.downloadClaim(name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else if token == nil {
					http.Error(w, "No such token", http.StatusNotFound)
				} else {
					writeJson(token, w)
				}

			} else {
				http.Error(w, "Token must be specified in path", http.StatusNotFound)
			}
		} else {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})
}
