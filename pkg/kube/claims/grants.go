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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils"
)

type GrantResponse func(namespace string, name string, subject string, writer io.Writer) error

type Grants struct {
	clients    internalclient.Clients
	generator  GrantResponse
	url        string
	ca         string
	scheme     string
	grants     map[kubetypes.UID]*skupperv1alpha1.AccessGrant
	grantIndex map[string]kubetypes.UID
	lock       sync.Mutex
}

func newGrants(clients internalclient.Clients, generator GrantResponse, scheme string, url string) *Grants {
	return &Grants{
		clients:    clients,
		generator:  generator,
		scheme:     scheme,
		url:        url,
		grants:     map[kubetypes.UID]*skupperv1alpha1.AccessGrant{},
		grantIndex: map[string]kubetypes.UID{},
	}
}

func (g *Grants) setCA(ca string) bool {
	g.lock.Lock()
	defer g.lock.Unlock()
	if g.ca == ca {
		return false
	}
	g.ca = ca
	return true
}

func (g *Grants) getCA() string {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.ca
}

func (g *Grants) record(key string, grant *skupperv1alpha1.AccessGrant) {
	g.lock.Lock()
	defer g.lock.Unlock()
	if uid, ok := g.grantIndex[key]; ok && uid != grant.ObjectMeta.UID {
		delete(g.grants, uid)
	}
	g.grantIndex[key] = grant.ObjectMeta.UID
	g.grants[grant.ObjectMeta.UID] = grant
}

func (g *Grants) remove(key string) {
	g.lock.Lock()
	defer g.lock.Unlock()
	if uid, ok := g.grantIndex[key]; ok {
		delete(g.grantIndex, key)
		delete(g.grants, uid)
	}
}

func (g *Grants) put(grant *skupperv1alpha1.AccessGrant) error {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.grants[grant.ObjectMeta.UID] = grant
	return nil
}

func (g *Grants) get(key string) *skupperv1alpha1.AccessGrant {
	g.lock.Lock()
	defer g.lock.Unlock()

	if grant, ok := g.grants[kubetypes.UID(key)]; ok {
		return grant
	}
	return nil
}

func (g *Grants) getAll() []*skupperv1alpha1.AccessGrant {
	g.lock.Lock()
	defer g.lock.Unlock()
	var grants []*skupperv1alpha1.AccessGrant
	for _, grant := range g.grants {
		grants = append(grants, grant)
	}
	return grants
}

func (g *Grants) setUrl(url string) bool {
	g.lock.Lock()
	defer g.lock.Unlock()
	if g.url == url {
		return false
	}
	g.url = url
	return true
}

func (g *Grants) getUrl() string {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.url
}

func (g *Grants) recheckUrl() {
	for _, grant := range g.getAll() {
		key := fmt.Sprintf("%s/%s", grant.Namespace, grant.Name)
		if changed, message := g.checkUrl(key, grant); changed {
			if message == "" {
				message = skupperv1alpha1.STATUS_OK
			}
			grant.SetResolved()
			if grant.Status.SetStatusMessage(message) {
				if err := g.updateGrantStatus(grant); err != nil {
					log.Printf("Error updating grant %s after setting url: %s", key, err)
				}
			}
		}
	}
}

func (g *Grants) recheckCa() {
	for _, grant := range g.getAll() {
		key := fmt.Sprintf("%s/%s", grant.Namespace, grant.Name)
		if g.checkCa(key, grant) {
			grant.SetResolved()
			if err := g.updateGrantStatus(grant); err != nil {
				log.Printf("Error updating grant %s after setting ca: %s", key, err)
			}
		}
	}
}

func (g *Grants) claimUrl(grant *skupperv1alpha1.AccessGrant) string {
	if url := g.getUrl(); url != "" {
		return fmt.Sprintf("%s://%s/%s", g.scheme, url, string(grant.ObjectMeta.UID))
	}
	return ""
}

func (g *Grants) checkUrl(key string, grant *skupperv1alpha1.AccessGrant) (bool, string) {
	url := g.claimUrl(grant)
	if url == "" {
		log.Printf("URL pending for AccessGrant %s", key)
		return true, "Url pending"
	} else if grant.Status.Url != url {
		log.Printf("Setting URL for AccessGrant %s to %s", key, url)
		grant.Status.Url = url
		return true, ""
	} else {
		return false, ""
	}
}

func (g *Grants) checkCa(key string, grant *skupperv1alpha1.AccessGrant) bool {
	ca := g.getCA()
	if grant.Status.Ca == ca {
		return false
	}
	grant.Status.Ca = ca
	return true
}

func (g *Grants) checkGrant(key string, grant *skupperv1alpha1.AccessGrant) error {
	if grant == nil {
		g.remove(key)
		return nil
	}
	g.record(key, grant)
	changed := false
	var status []string
	if updated, message := g.checkUrl(key, grant); updated {
		changed = true
		if message != "" {
			status = append(status, message)
		}
	}
	if g.checkCa(key, grant) {
		changed = true
	}

	if grant.Status.Code == "" {
		if grant.Spec.Code == "" {
			grant.Status.Code = utils.RandomId(24)
		} else {
			grant.Status.Code = grant.Spec.Code
		}
	}

	if grant.Status.Expiration == "" {
		if grant.Spec.ExpirationWindow != "" {
			d, e := time.ParseDuration(grant.Spec.ExpirationWindow)
			if e != nil {
				status = append(status, fmt.Sprintf("Invalid duration %q: %s", grant.Spec.ExpirationWindow, e))
			} else {
				expiration := time.Now().Add(d).Format(time.RFC3339)
				if grant.Status.Expiration != expiration {
					grant.Status.Expiration = expiration
					changed = true
				}
			}
		} else {
			grant.Status.Expiration = time.Now().Add(time.Minute * 10).Format(time.RFC3339)
			changed = true
		}
	}
	var err error

	if len(status) != 0 {
		err = fmt.Errorf(strings.Join(status, ", "))
	}
	if grant.SetProcessed(err) {
		changed = true
	}
	if grant.SetResolved() {
		changed = true
	}

	if !changed {
		return nil
	}
	log.Printf("Updating status for AccessGrant %s", key)
	return g.updateGrantStatus(grant)
}

func (g *Grants) updateGrantStatus(grant *skupperv1alpha1.AccessGrant) error {
	updated, err := g.clients.GetSkupperClient().SkupperV1alpha1().AccessGrants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	g.put(updated)
	return nil
}

func (g *Grants) checkAndUpdateAccessToken(key string, data []byte) (*skupperv1alpha1.AccessGrant, *HttpError) {
	log.Printf("Checking access token for %s", key)
	grant := g.get(key)
	if grant == nil {
		return nil, httpError("No such claim", http.StatusNotFound)
	}

	expiration, err := time.Parse(time.RFC3339, grant.Status.Expiration)
	if err != nil {
		log.Printf("Cannot determine expiration for %s/%s: %s", grant.Namespace, grant.Name, err)
		return nil, httpError("Corrupted claim", http.StatusInternalServerError)
	}
	if expiration.Before(time.Now()) {
		log.Printf("AccessGrant %s/%s expired", grant.Namespace, grant.Name)
		return nil, httpError("No such claim", http.StatusNotFound)
	}
	if grant.Spec.RedemptionsAllowed <= grant.Status.Redeemed {
		log.Printf("AccessGrant %s/%s already redeemed", grant.Namespace, grant.Name)
		return nil, httpError("No such access granted", http.StatusNotFound)
	}
	if grant.Status.Code != string(data) {
		return nil, httpError("Redemption of access token refused", http.StatusForbidden)
	}
	grant.Status.Redeemed += 1
	err = g.updateGrantStatus(grant)
	if err != nil {
		log.Printf("Error updating access grant %s/%s: %s", grant.Namespace, grant.Name, err)
		return nil, httpError("Internal error", http.StatusServiceUnavailable)
	}
	return grant, nil
}

func (g *Grants) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Bad method %s for path %s", r.Method, r.URL.Path)
		http.Error(w, "Only POST is supported", http.StatusMethodNotAllowed)
		return
	}
	key := strings.Join(strings.Split(r.URL.Path, "/"), "")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body for path %s: %s", r.URL.Path, err.Error())
		http.Error(w, "Request body not valid", http.StatusBadRequest)
		return
	}

	grant, e := g.checkAndUpdateAccessToken(key, body)
	if e != nil {
		e.write(w)
		return
	}

	name := r.Header.Get("name")
	if name == "" {
		log.Printf("No name specified when redeeming access token for %s/%s, using access grant name", grant.Namespace, grant.Name)
		name = grant.Name
	}
	subject := r.Header.Get("subject")
	if subject == "" {
		subject = name
	}
	if err := g.generator(grant.Namespace, name, subject, w); err != nil {
		log.Printf("Failed to create token for %s/%s: %s", grant.Namespace, grant.Name, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Redemption of access token %s/%s succeeded", grant.Namespace, grant.Name)
}

type HttpError struct {
	text string
	code int
}

func (e *HttpError) write(w http.ResponseWriter) {
	http.Error(w, e.text, e.code)
}

func httpError(text string, code int) *HttpError {
	return &HttpError{
		text: text,
		code: code,
	}
}
