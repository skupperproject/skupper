package grants

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type GrantResponse func(namespace string, name string, subject string, writer io.Writer) error

type Grants struct {
	clients    internalclient.Clients
	generator  GrantResponse
	url        string
	ca         string
	scheme     string
	grants     map[kubetypes.UID]*skupperv2alpha1.AccessGrant
	grantIndex map[string]kubetypes.UID
	lock       sync.Mutex
	logger     *slog.Logger
}

func newGrants(clients internalclient.Clients, generator GrantResponse, scheme string, url string) *Grants {
	return &Grants{
		clients:    clients,
		generator:  generator,
		scheme:     scheme,
		url:        url,
		grants:     map[kubetypes.UID]*skupperv2alpha1.AccessGrant{},
		grantIndex: map[string]kubetypes.UID{},
		logger:     slog.New(slog.Default().Handler()).With(slog.String("component", "kube.grants")),
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

func (g *Grants) record(key string, grant *skupperv2alpha1.AccessGrant) {
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

func (g *Grants) put(grant *skupperv2alpha1.AccessGrant) error {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.grants[grant.ObjectMeta.UID] = grant
	return nil
}

func (g *Grants) get(key string) *skupperv2alpha1.AccessGrant {
	g.lock.Lock()
	defer g.lock.Unlock()

	if grant, ok := g.grants[kubetypes.UID(key)]; ok {
		return grant
	}
	return nil
}

func (g *Grants) getAll() []*skupperv2alpha1.AccessGrant {
	g.lock.Lock()
	defer g.lock.Unlock()
	var grants []*skupperv2alpha1.AccessGrant
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
		if g.checkUrl(key, grant) {
			grant.SetResolved()
			if err := g.updateGrantStatus(grant); err != nil {
				g.logger.Error("Error updating grant after setting url", slog.String("key", key), slog.Any("error", err))
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
				g.logger.Error("Error updating grant after setting ca", slog.String("key", key), slog.Any("error", err))
			}
		}
	}
}

func (g *Grants) claimUrl(grant *skupperv2alpha1.AccessGrant) string {
	if url := g.getUrl(); url != "" {
		return fmt.Sprintf("%s://%s/%s", g.scheme, url, string(grant.ObjectMeta.UID))
	}
	return ""
}

func (g *Grants) checkUrl(key string, grant *skupperv2alpha1.AccessGrant) bool {
	url := g.claimUrl(grant)
	if grant.Status.Url == url {
		return false
	}
	g.logger.Info("Setting URL for AccessGrant", slog.String("accessGrant", key), slog.String("url", url))
	grant.Status.Url = url
	return true
}

func (g *Grants) checkCa(key string, grant *skupperv2alpha1.AccessGrant) bool {
	ca := g.getCA()
	if grant.Status.Ca == ca {
		return false
	}
	grant.Status.Ca = ca
	return true
}

func (g *Grants) checkGrant(key string, grant *skupperv2alpha1.AccessGrant) error {
	if grant == nil {
		g.remove(key)
		return nil
	}

	// if RedemptionsAllowed is not set default to 1 so that grant is usable
	if grant.Spec.RedemptionsAllowed == 0 {
		grant.Spec.RedemptionsAllowed = 1
		_, e := g.clients.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.ObjectMeta.Namespace).Update(context.TODO(), grant, metav1.UpdateOptions{})
		if e != nil {
			return fmt.Errorf("%s", fmt.Sprintf("Failed updating Redemptions Allowed %s", e))
		} else {
			return nil
		}
	}

	g.record(key, grant)
	changed := false
	var status []string
	if g.checkUrl(key, grant) {
		changed = true
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

	if grant.Status.ExpirationTime == "" {
		if grant.Spec.ExpirationWindow != "" {
			d, e := time.ParseDuration(grant.Spec.ExpirationWindow)
			if e != nil {
				status = append(status, fmt.Sprintf("Invalid duration %q: %s", grant.Spec.ExpirationWindow, e))
			} else {
				expiration := time.Now().Add(d).Format(time.RFC3339)
				if grant.Status.ExpirationTime != expiration {
					grant.Status.ExpirationTime = expiration
					changed = true
				}
			}
		} else {
			grant.Status.ExpirationTime = time.Now().Add(time.Minute * 10).Format(time.RFC3339)
			changed = true
		}
	}

	var err error

	// if RedemptionsAllowed is not set default to 1 so that grant is usable
	if grant.Spec.RedemptionsAllowed == 0 {
		grant.Spec.RedemptionsAllowed = 1
		changed = true
	}

	if len(status) != 0 {
		err = fmt.Errorf("%s", strings.Join(status, ", "))
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
	return g.updateGrantStatus(grant)
}

func (g *Grants) updateGrantStatus(grant *skupperv2alpha1.AccessGrant) error {
	updated, err := g.clients.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	g.put(updated)
	return nil
}

func (g *Grants) checkAndUpdateAccessToken(key string, data []byte) (*skupperv2alpha1.AccessGrant, *HttpError) {
	g.logger.Info("Checking access token", slog.String("key", key))
	grant := g.get(key)
	if grant == nil {
		return nil, httpError("No such claim", http.StatusNotFound)
	}

	expiration, err := time.Parse(time.RFC3339, grant.Status.ExpirationTime)
	if err != nil {
		g.logger.Error("Cannot determine expiration for grant",
			slog.String("namespace", grant.Namespace),
			slog.String("name", grant.Name),
			slog.Any("error", err))
		return nil, httpError("Corrupted claim", http.StatusInternalServerError)
	}
	if expiration.Before(time.Now()) {
		g.logger.Info("AccessGrant expired", slog.String("namespace", grant.Namespace), slog.String("name", grant.Name))
		return nil, httpError("No such claim", http.StatusNotFound)
	}
	if grant.Spec.RedemptionsAllowed <= grant.Status.Redemptions {
		g.logger.Info("AccessGrant already redeemed", slog.String("namespace", grant.Namespace), slog.String("name", grant.Name))
		return nil, httpError("No such access granted", http.StatusNotFound)
	}
	if grant.Status.Code != string(data) {
		return nil, httpError("Redemption of access token refused", http.StatusForbidden)
	}
	grant.Status.Redemptions += 1
	err = g.updateGrantStatus(grant)
	if err != nil {
		g.logger.Error("Error updating access grant",
			slog.String("namespace", grant.Namespace),
			slog.String("name", grant.Name),
			slog.Any("error", err))
		return nil, httpError("Internal error", http.StatusServiceUnavailable)
	}
	return grant, nil
}

func (g *Grants) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.logger.Error("Bad method for path", slog.String("method", r.Method), slog.String("path", r.URL.Path))
		http.Error(w, "Only POST is supported", http.StatusMethodNotAllowed)
		return
	}
	key := strings.Join(strings.Split(r.URL.Path, "/"), "")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		g.logger.Error("Error reading body for path", slog.String("path", r.URL.Path), slog.Any("error", err))
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
		g.logger.Info("No name specified when redeeming access token using access grant name",
			slog.String("namespace", grant.Namespace),
			slog.String("name", grant.Name))
		name = grant.Name
	}
	subject := r.Header.Get("subject")
	if subject == "" {
		subject = name
	}
	if err := g.generator(grant.Namespace, name, subject, w); err != nil {
		g.logger.Error("Failed to create token for grant",
			slog.String("namespace", grant.Namespace),
			slog.String("name", grant.Name),
			slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	g.logger.Info("Redemption of access token succeeded", slog.String("namespace", grant.Namespace), slog.String("name", grant.Name))
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
