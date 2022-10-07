package domain

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

type LinkHandler interface {
	Create(secret *corev1.Secret, name string, cost int) error
	Delete(name string) error
	List() ([]*corev1.Secret, error)
	StatusAll() ([]types.LinkStatus, error)
	Status(name string) (types.LinkStatus, error)
	Detail(link types.LinkStatus) (map[string]string, error)
	RemoteLinks(ctx context.Context) ([]*types.RemoteLinkInfo, error)
}

type SecretUpdateFn func(claim *corev1.Secret) error
type EventLogger func(name string, format string, args ...interface{})

type ClaimRedeemer struct {
	siteId      string
	siteVersion string
	updateFn    SecretUpdateFn
	name        string
	logger      EventLogger
}

func NewClaimRedeemer(name, siteId, siteVersion string, secretUpdater SecretUpdateFn, event EventLogger) *ClaimRedeemer {
	return &ClaimRedeemer{
		name:        name,
		siteId:      siteId,
		siteVersion: siteVersion,
		updateFn:    secretUpdater,
		logger:      event,
	}
}

func (c *ClaimRedeemer) handleError(claim *corev1.Secret, text string, failed bool) error {
	if failed {
		if claim.ObjectMeta.Annotations == nil {
			claim.ObjectMeta.Annotations = map[string]string{}
		}
		claim.ObjectMeta.Annotations[types.LastFailedAnnotationKey] = time.Now().Format(time.RFC3339)
	}
	claim.ObjectMeta.Annotations[types.StatusAnnotationKey] = text
	err := c.updateFn(claim)
	if err != nil {
		c.logger("ClaimRedeemer", "Failed to update status for claim %q: %s", claim.ObjectMeta.Name, err)
	}
	if !failed {
		return fmt.Errorf("Error processing claim %q: %s", claim.ObjectMeta.Name, text)
	} else {
		c.logger(c.name, "Failed to process claim %q: %s", claim.ObjectMeta.Name, text)
		return nil
	}
}
func (c *ClaimRedeemer) RedeemClaim(claim *corev1.Secret) error {

	if claim.ObjectMeta.Annotations == nil {
		return c.handleError(claim, "no annotations", true)
	}
	if claim.Data == nil {
		return c.handleError(claim, "no data", true)
	}
	url, ok := claim.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey]
	if !ok {
		return c.handleError(claim, "no url specified", true)
	}
	password, ok := claim.Data[types.ClaimPasswordDataKey]
	if !ok {
		return c.handleError(claim, "no password specified", true)
	}
	if failed, ok := claim.ObjectMeta.Annotations[types.LastFailedAnnotationKey]; ok {
		c.logger(c.name, "Skipping failed claim %q (failed at %s)", claim.ObjectMeta.Name, failed)
		return nil
	}

	ca, ok := claim.Data[types.ClaimCaCertDataKey]
	transport := &http.Transport{}
	if ok {
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM(ca)
		transport.TLSClientConfig = &tls.Config{
			RootCAs: caPool,
		}
	}
	client := &http.Client{
		Transport: transport,
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(password))
	request.Header.Add("skupper-site-name", c.siteId)
	query := request.URL.Query()
	query.Add("site-version", c.siteVersion)
	request.URL.RawQuery = query.Encode()
	response, err := client.Do(request)
	if err != nil {
		return c.handleError(claim, err.Error(), false)
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return c.handleError(claim, err.Error(), false)
	}
	if response.StatusCode != http.StatusOK {
		fmt.Printf("Claim request failed with code: %d", response.StatusCode)
		fmt.Println()
		return c.handleError(claim, strings.TrimSpace(string(body)), response.StatusCode == http.StatusNotFound)
	}
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	var token corev1.Secret
	_, _, err = s.Decode(body, nil, &token)
	if err != nil {
		return c.handleError(claim, "could not parse connection token", false)
	}
	for key, value := range token.ObjectMeta.Annotations {
		claim.ObjectMeta.Annotations[key] = value
	}
	for key, value := range token.ObjectMeta.Labels {
		claim.ObjectMeta.Labels[key] = value
	}
	claim.Data = token.Data
	err = c.updateFn(claim)
	if err != nil {
		return fmt.Errorf("Could not store connection token for claim %q: %s", claim.ObjectMeta.Name, err)
	}
	c.logger(c.name, "Retrieved token %s from %s", token.ObjectMeta.Name, url)
	return nil
}

func VerifyNotSelfOrDuplicate(secret corev1.Secret, self string, linkHandler LinkHandler) error {
	if secret.ObjectMeta.Annotations == nil {
		return fmt.Errorf("The secret has not annotations")
	}
	generatedBy, ok := secret.ObjectMeta.Annotations[types.TokenGeneratedBy]
	if !ok {
		return fmt.Errorf("Can't find secret origin.")
	}
	if self == string(generatedBy) {
		return fmt.Errorf("Can't create connection to self with token")
	}
	currentSecrets, err := linkHandler.List()
	if err != nil {
		return fmt.Errorf("Could not retrieve secrets: %w", err)
	}
	for _, currentSecret := range currentSecrets {
		currentAuthor, ok := currentSecret.Annotations[types.TokenGeneratedBy]
		if !ok {
			return fmt.Errorf("A secret has no author.")
		}
		if generatedBy == currentAuthor {
			return fmt.Errorf("Already connected to \"%s\".", currentAuthor)
		}
	}
	return nil
}

// VerifySecretCompatibility returns nil if current site version is compatible
// with the token or cert provided. If sites are not compatible an error is
// returned with the appropriate information
func VerifySecretCompatibility(localSiteVersion string, secret corev1.Secret) error {
	var secretVersion string
	if secret.Annotations != nil {
		secretVersion = secret.Annotations[types.SiteVersion]
	}
	if err := VerifySiteCompatibility(localSiteVersion, secretVersion); err != nil {
		if secretVersion == "" {
			secretVersion = "undefined"
		}
		return fmt.Errorf("%v - remote site version is %s", err, secretVersion)
	}
	return nil
}

func GenerateLinkName(linkHandler LinkHandler) string {
	secrets, err := linkHandler.List()
	max := 1
	if err == nil {
		connector_name_pattern := regexp.MustCompile("link([0-9]+)+")
		for _, s := range secrets {
			count := connector_name_pattern.FindStringSubmatch(s.ObjectMeta.Name)
			if len(count) > 1 {
				v, _ := strconv.Atoi(count[1])
				if v >= max {
					max = v + 1
				}
			}

		}
	} else {
		log.Fatal("Could not retrieve token secrets:", err)
	}
	return "link" + strconv.Itoa(max)
}

func GetTokenEndpoint(site Site, secret *corev1.Secret) (string, string) {
	var hostname string
	var port string
	if secret.ObjectMeta.Annotations == nil {
		secret.ObjectMeta.Annotations = map[string]string{}
	}
	if site.GetMode() == string(types.TransportModeEdge) {
		hostname = secret.ObjectMeta.Annotations["edge-host"]
		port = secret.ObjectMeta.Annotations["edge-port"]
	} else {
		hostname = secret.ObjectMeta.Annotations["inter-router-host"]
		port = secret.ObjectMeta.Annotations["inter-router-port"]
	}
	return hostname, port
}
