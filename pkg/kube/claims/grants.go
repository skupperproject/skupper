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

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
)

type GrantResponse func(namespace string, name string, writer io.Writer) error

type Grants struct {
	clients     kube.Clients
	generator   GrantResponse
	url         string
	ca          string
	grants      map[kubetypes.UID]*skupperv1alpha1.Grant
	grantIndex  map[string]kubetypes.UID
	lock        sync.Mutex
}

func newGrants(clients kube.Clients, generator GrantResponse, url string) *Grants {
	return &Grants{
		clients:     clients,
		generator:   generator,
		url:         url,
		grants:      map[kubetypes.UID]*skupperv1alpha1.Grant{},
		grantIndex:  map[string]kubetypes.UID{},
	}
}

func (g *Grants) record(key string, grant *skupperv1alpha1.Grant) {
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

func (g *Grants) put(grant *skupperv1alpha1.Grant) error {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.grants[grant.ObjectMeta.UID] = grant
	return nil
}

func (g *Grants) get(key string) *skupperv1alpha1.Grant {
	g.lock.Lock()
	defer g.lock.Unlock()

	if grant, ok := g.grants[kubetypes.UID(key)]; ok {
		return grant
	}
	return nil
}

func (g *Grants) checkGrant(key string, grant *skupperv1alpha1.Grant) error {
	if grant == nil {
		g.remove(key)
		return nil
	}
	g.record(key, grant)
	changed := false
	var status []string
	if g.url == "" {
		status = append(status, "Url pending")
	} else if grant.Status.Url != g.url {
		grant.Status.Url = g.url
		changed = true
	}
	if grant.Status.Ca != g.ca {
		grant.Status.Ca = g.ca
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
	return g.updateGrantStatus(grant)
}

func (g *Grants) updateGrantStatus(grant *skupperv1alpha1.Grant) error {
	updated, err := g.clients.GetSkupperClient().SkupperV1alpha1().Grants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	g.put(updated)
	return nil
}

func (g *Grants) checkAndUpdateClaim(key string, data []byte) (*skupperv1alpha1.Grant, *HttpError) {
	log.Printf("Checking claim for %s", key)
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
		log.Printf("Grant %s/%s expired", grant.Namespace, grant.Name)
		return nil, httpError("No such claim", http.StatusNotFound)
	}
	if grant.Spec.Claims <= grant.Status.Claimed {
		log.Printf("Grant %s/%s already claimed", grant.Namespace, grant.Name)
		return nil, httpError("No such claim", http.StatusNotFound)
	}
	if grant.Status.Secret != string(data) {
		return nil, httpError("Claim refused", http.StatusForbidden)
	}
	grant.Status.Claimed += 1
	err = g.updateGrantStatus(grant)
	if err != nil {
		log.Printf("Error updating grant %s/%s: %s", grant.Namespace, grant.Name, err)
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

	grant, e := g.checkAndUpdateClaim(key, body)
	if e != nil {
		e.write(w)
		return
	}

	name := r.Header.Get("name")
	if name == "" {
		log.Printf("No name specified when redeeming claim for %s/%s, using grant name", grant.Namespace, grant.Name)
		name = grant.Name
	}
	if err := g.generator(grant.Namespace, name, w); err != nil {
		log.Printf("Failed to create token for %s/%s: %s", grant.Namespace, grant.Name, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Claim for %s/%s succeeded", grant.Namespace, grant.Name)
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
