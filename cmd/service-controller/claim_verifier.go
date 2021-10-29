package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
)

const (
	TokenClaimVerification string = "TokenClaimVerification"
)

type TokenGenerator interface {
	ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error)
}

type ClaimVerifier struct {
	vanClient *client.VanClient
}

func newClaimVerifier(client *client.VanClient) *ClaimVerifier {
	return &ClaimVerifier{
		vanClient: client,
	}
}

func alwaysRetriable(err error) bool {
	return true
}

func (server *ClaimVerifier) checkAndUpdateClaim(name string, data []byte) (string, int) {
	claim, err := server.vanClient.KubeClient.CoreV1().Secrets(server.vanClient.Namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return "No such claim", http.StatusNotFound
	} else if err != nil {
		return err.Error(), http.StatusInternalServerError
	}
	if claim.ObjectMeta.Labels == nil || claim.ObjectMeta.Labels[types.SkupperTypeQualifier] != types.TypeClaimRecord {
		return "No such claim", http.StatusNotFound
	}
	if claim.ObjectMeta.Annotations != nil {
		if expirationString, ok := claim.ObjectMeta.Annotations[types.ClaimExpiration]; ok {
			expiration, err := time.Parse(time.RFC3339, expirationString)
			if err != nil {
				event.Recordf(TokenClaimVerification, "Cannot determine expiration: %s", err)
				return "Corrupted claim", http.StatusInternalServerError
			} else if expiration.Before(time.Now()) {
				event.Recordf(TokenClaimVerification, "Claim %s expired", name)
				return "No such claim", http.StatusNotFound
			}
		}
	}
	if !bytes.Equal(claim.Data["password"], data) {
		return "Claim refused", http.StatusForbidden
	}
	if claim.ObjectMeta.Annotations == nil {
		claim.ObjectMeta.Annotations = map[string]string{}
	}
	if uses, ok := claim.ObjectMeta.Annotations[types.ClaimsRemaining]; ok {
		remainingUses, err := strconv.Atoi(uses)
		if err != nil {
			event.Recordf(TokenClaimVerification, "Cannot determine remaining uses: %s", err)
			return "Corrupted claim", http.StatusInternalServerError
		}
		if remainingUses == 0 {
			event.Recordf(TokenClaimVerification, "Claim %s already used", name)
			return "No such claim", http.StatusNotFound
		}
		remainingUses -= 1
		claim.ObjectMeta.Annotations[types.ClaimsRemaining] = strconv.Itoa(remainingUses)
	}
	if value, ok := claim.ObjectMeta.Annotations[types.ClaimsMade]; ok {
		made, err := strconv.Atoi(value)
		if err != nil {
			event.Recordf(TokenClaimVerification, "Cannot determine claims made: %s", err)
			return "Corrupted claim", http.StatusInternalServerError
		}
		made += 1
		claim.ObjectMeta.Annotations[types.ClaimsMade] = strconv.Itoa(made)
	} else {
		claim.ObjectMeta.Annotations[types.ClaimsMade] = "1"
	}
	_, err = server.vanClient.KubeClient.CoreV1().Secrets(server.vanClient.Namespace).Update(claim)
	if err != nil {
		event.Recordf(TokenClaimVerification, "Error updating remaining uses: %s", err)
		return "Internal error", http.StatusServiceUnavailable
	}
	return "ok", http.StatusOK
}

func (server *ClaimVerifier) redeemClaim(name string, subject string, data []byte, generator TokenGenerator) (*corev1.Secret, string, int) {
	text := ""
	code := http.StatusServiceUnavailable
	backoff := retry.DefaultRetry
	for i := 0; i < 5 && code == http.StatusServiceUnavailable; i++ {
		if i > 0 {
			time.Sleep(backoff.Step())
		}
		text, code = server.checkAndUpdateClaim(name, data)
	}
	if code != http.StatusOK {
		return nil, text, code
	}
	token, _, err := generator.ConnectorTokenCreate(context.TODO(), subject, "")
	if err != nil {
		return nil, err.Error(), http.StatusInternalServerError
	}
	return token, "ok", http.StatusOK

}

func (server *ClaimVerifier) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		event.Recordf(TokenClaimVerification, "Bad method %s", r.Method)
		http.Error(w, "Only POST is supported", http.StatusMethodNotAllowed)
		return
	}
	name := strings.Join(strings.Split(r.URL.Path, "/"), "")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		event.Record(TokenClaimVerification, err.Error())
		http.Error(w, "Request body not valid", http.StatusBadRequest)
		return
	}
	subject := r.Header.Get("skupper-site-name")
	if subject == "" {
		log.Printf("No site name specified, using claim name")
		subject = name
	}
	remoteSiteVersion := r.URL.Query().Get("site-version")
	if err = server.vanClient.VerifySiteCompatibility(remoteSiteVersion); err != nil {
		if remoteSiteVersion == "" {
			remoteSiteVersion = "undefined"
		}
		event.Recordf(TokenClaimVerification, "%s - remote site version is %s", err.Error(), remoteSiteVersion)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, text, code := server.redeemClaim(name, subject, body, server.vanClient)
	if token == nil {
		event.Recordf(TokenClaimVerification, "Claim request for %s failed: %s", name, text)
		http.Error(w, text, code)
		return
	}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	err = s.Encode(token, w)
	if err != nil {
		event.Record(TokenClaimVerification, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	event.Recordf(TokenClaimVerification, "Claim for %s succeeded", name)
}

func (server *ClaimVerifier) start(stopCh <-chan struct{}) error {
	go server.listen()
	return nil
}

func enableClaimVerifier() bool {
	mode := os.Getenv("SKUPPER_ROUTER_MODE")
	return mode != string(types.TransportModeEdge)
}

func (server *ClaimVerifier) listen() {
	addr := fmt.Sprintf(":%d", types.ClaimRedemptionPort)
	log.Printf("Claim verifier listening on %s", addr)
	log.Fatal(http.ListenAndServeTLS(addr, "/etc/service-controller/certs/tls.crt", "/etc/service-controller/certs/tls.key", server))
}
