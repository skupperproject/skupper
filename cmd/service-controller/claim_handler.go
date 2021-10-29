package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
)

type ClaimHandler struct {
	name      string
	vanClient *client.VanClient
	siteId    string
}

func (h *ClaimHandler) Handle(name string, claim *corev1.Secret) error {
	if claim != nil {
		return h.redeemClaim(claim)
	}
	return nil
}

func newClaimHandler(cli *client.VanClient, siteId string) *SecretController {
	handler := &ClaimHandler{
		name:      "ClaimHandler",
		vanClient: cli,
		siteId:    siteId,
	}
	return NewSecretController(handler.name, types.ClaimRequestSelector, cli.KubeClient, cli.Namespace, handler)
}

func (h *ClaimHandler) handleError(claim *corev1.Secret, text string, failed bool) error {
	if failed {
		if claim.ObjectMeta.Annotations == nil {
			claim.ObjectMeta.Annotations = map[string]string{}
		}
		claim.ObjectMeta.Annotations[types.LastFailedAnnotationKey] = time.Now().Format(time.RFC3339)
	}
	claim.ObjectMeta.Annotations[types.StatusAnnotationKey] = text
	_, err := h.vanClient.KubeClient.CoreV1().Secrets(h.vanClient.Namespace).Update(claim)
	if err != nil {
		event.Recordf(h.name, "Failed to update status for claim %q: %s", claim.ObjectMeta.Name, err)
	}
	if !failed {
		return fmt.Errorf("Error processing claim %q: %s", claim.ObjectMeta.Name, text)
	} else {
		event.Recordf(h.name, "Failed to process claim %q: %s", claim.ObjectMeta.Name, text)
		return nil
	}
}

func (h *ClaimHandler) redeemClaim(claim *corev1.Secret) error {
	if claim.ObjectMeta.Annotations == nil {
		return h.handleError(claim, "no annotations", true)
	}
	if claim.Data == nil {
		return h.handleError(claim, "no data", true)
	}
	url, ok := claim.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey]
	if !ok {
		return h.handleError(claim, "no url specified", true)
	}
	password, ok := claim.Data[types.ClaimPasswordDataKey]
	if !ok {
		return h.handleError(claim, "no password specified", true)
	}
	if failed, ok := claim.ObjectMeta.Annotations[types.LastFailedAnnotationKey]; ok {
		event.Recordf(h.name, "Skipping failed claim %q (failed at %s)", claim.ObjectMeta.Name, failed)
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
	siteMeta, err := h.vanClient.GetSiteMetadata()
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(password))
	request.Header.Add("skupper-site-name", h.siteId)
	query := request.URL.Query()
	query.Add("site-version", siteMeta.Version)
	request.URL.RawQuery = query.Encode()
	response, err := client.Do(request)
	if err != nil {
		return h.handleError(claim, err.Error(), false)
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return h.handleError(claim, err.Error(), false)
	}
	if response.StatusCode != http.StatusOK {
		fmt.Printf("Claim request failed with code: %d", response.StatusCode)
		fmt.Println()
		return h.handleError(claim, strings.TrimSpace(string(body)), response.StatusCode == http.StatusNotFound)
	}
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	var token corev1.Secret
	_, _, err = s.Decode(body, nil, &token)
	if err != nil {
		return h.handleError(claim, "could not parse connection token", false)
	}
	for key, value := range token.ObjectMeta.Annotations {
		claim.ObjectMeta.Annotations[key] = value
	}
	for key, value := range token.ObjectMeta.Labels {
		claim.ObjectMeta.Labels[key] = value
	}
	claim.Data = token.Data
	_, err = h.vanClient.KubeClient.CoreV1().Secrets(h.vanClient.Namespace).Update(claim)
	if err != nil {
		return fmt.Errorf("Could not store connection token for claim %q: %s", claim.ObjectMeta.Name, err)
	}
	event.Recordf(h.name, "Retrieved token %q from %q", token.ObjectMeta.Name, url)
	return nil
}
