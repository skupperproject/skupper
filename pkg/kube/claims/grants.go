package claims

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
)

type Grants struct {
	clients     kube.Clients
	generator   TokenGenerator
	siteChecker SiteChecker
	grants      map[kubetypes.UID]*skupperv1alpha1.Grant
	grantIndex  map[string]kubetypes.UID
	ca          string
	url         string
	addr        string
	cert        string
	key         string
	lock        sync.Mutex
}

func NewGrants(clients kube.Clients, generator TokenGenerator, siteChecker SiteChecker) *Grants {
	return &Grants{
		clients:     clients,
		generator:   generator,
		siteChecker: siteChecker,
		grants:      map[kubetypes.UID]*skupperv1alpha1.Grant{},
		grantIndex:  map[string]kubetypes.UID{},
	}
}

func (server *Grants) GrantDeleted(key string) error {
	server.lock.Lock()
	defer server.lock.Unlock()
	if uid, ok := server.grantIndex[key]; ok {
		delete(server.grantIndex, key)
		delete(server.grants, uid)
	}
	return nil
}

func (server *Grants) GrantChanged(key string, grant *skupperv1alpha1.Grant) error {
	server.lock.Lock()
	defer server.lock.Unlock()
	if uid, ok := server.grantIndex[key]; ok && uid != grant.ObjectMeta.UID {
		delete(server.grants, uid)
	}
	server.grantIndex[key] = grant.ObjectMeta.UID
	server.grants[grant.ObjectMeta.UID] = grant
	changed := false
	var status []string
	//TODO: Url and Ca come from configuration
	if grant.Status.Url != server.url {
		grant.Status.Url = server.url
		changed = true
	}
	if grant.Status.Ca != server.ca {
		grant.Status.Ca = server.ca
		changed = true
	}

	if grant.Status.Secret == "" {
		if grant.Spec.Secret == "" {
			grant.Status.Secret = utils.RandomId(24)
		} else {
			grant.Status.Secret = grant.Spec.Secret
		}
	}

	if grant.Spec.ValidFor != "" {
		d, e := time.ParseDuration(grant.Spec.ValidFor)
		if e != nil {
			status = append(status, fmt.Sprintf("Invalid duration %q: %s", grant.Spec.ValidFor, e))
		} else {
			expiration := time.Now().Add(d).Format(time.RFC3339)
			if grant.Status.Expiration != expiration {
				grant.Status.Expiration = expiration
				changed = true
			}
		}
	} else if grant.Status.Expiration == "" {
		grant.Status.Expiration = time.Now().Add(time.Minute * 10).Format(time.RFC3339)
		changed = true
	}

	if len(status) != 0 {
		grant.Status.Status = strings.Join(status, ", ")
		changed = true
	} else if grant.Status.Status == "" {
		grant.Status.Status = "Ok"
		changed = true
	}

	if !changed {
		return nil
	}
	return server.updateGrantStatus(grant)
}

func (server *Grants) updateGrantStatus(grant *skupperv1alpha1.Grant) error {
	updated, err := server.clients.GetSkupperClient().SkupperV1alpha1().Grants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	server.grants[grant.ObjectMeta.UID] = updated
	return nil
}

func (server *Grants) checkAndUpdateClaim(key string, data []byte) (string, int) {
	log.Printf("Checking claim for %s", key)

	server.lock.Lock()
	defer server.lock.Unlock()

	grant, ok := server.grants[kubetypes.UID(key)]
	if !ok {
		return "No such claim", http.StatusNotFound
	}
	expiration, err := time.Parse(time.RFC3339, grant.Status.Expiration)
	if err != nil {
		log.Printf("Cannot determine expiration for %s/%s: %s", grant.Namespace, grant.Name, err)
		return "Corrupted claim", http.StatusInternalServerError
	} else if expiration.Before(time.Now()) {
		log.Printf("Grant %s/%s expired", grant.Namespace, grant.Name)
		return "No such claim", http.StatusNotFound
	}
	if grant.Spec.Claims <= grant.Status.Claimed {
		log.Printf("Grant %s/%s already claimed", grant.Namespace, grant.Name)
		return "No such claim", http.StatusNotFound
	}
	if grant.Status.Secret != string(data) {
		return "Claim refused", http.StatusForbidden
	}
	grant.Status.Claimed += 1
	err = server.updateGrantStatus(grant)
	if err != nil {
		log.Printf("Error updating grant %s/%s: %s", grant.Namespace, grant.Name, err)
		return "Internal error", http.StatusServiceUnavailable
	}
	return "ok", http.StatusOK
}

func (server *Grants) redeemClaim(name string, subject string, data []byte, generator TokenGenerator) (*corev1.Secret, string, int) {
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
		log.Printf("failed to check and update claim record: %s", text)
		return nil, text, code
	}
	token, _, err := generator.ConnectorTokenCreate(context.TODO(), subject, "")
	if err != nil {
		log.Printf("Failed to create token: %s", err.Error())
		return nil, err.Error(), http.StatusInternalServerError
	}
	return token, "ok", http.StatusOK

}

func (server *Grants) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Bad method %s", r.Method)
		http.Error(w, "Only POST is supported", http.StatusMethodNotAllowed)
		return
	}
	name := strings.Join(strings.Split(r.URL.Path, "/"), "")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %s", err.Error())
		http.Error(w, "Request body not valid", http.StatusBadRequest)
		return
	}
	subject := r.Header.Get("skupper-site-name")
	if subject == "" {
		log.Printf("No site name specified, using claim name")
		subject = name
	}
	remoteSiteVersion := r.URL.Query().Get("site-version")
	if err = server.siteChecker.VerifySiteCompatibility(remoteSiteVersion); err != nil {
		if remoteSiteVersion == "" {
			remoteSiteVersion = "undefined"
		}
		log.Printf("%s - remote site version is %s", err.Error(), remoteSiteVersion)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, text, code := server.redeemClaim(name, subject, body, server.generator)
	if token == nil {
		log.Printf("Claim request for %s failed: %s", name, text)
		http.Error(w, text, code)
		return
	}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	err = s.Encode(token, w)
	if err != nil {
		log.Printf("Error encoding token: %s", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Claim for %s succeeded", name)
}

func (server *Grants) Listen() {
	go server.listen()
}

func (server *Grants) listen() {
	err := http.ListenAndServeTLS(server.addr, server.cert, server.key, server)
	if err != nil {
		log.Printf("Claim verifier failed to start on %s: %s", server.addr, err)
	} else {
		log.Printf("Claim verifier listening on %s", server.addr)
	}
}
