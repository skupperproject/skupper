package grants

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func RedeemAccessToken(token *skupperv2alpha1.AccessToken, site *skupperv2alpha1.Site, clients internalclient.Clients) error {
	transport := &http.Transport{
		TLSClientConfig: tlsConfig(token),
	}
	body, err := postTokenRequest(token, site, transport)
	if err != nil {
		return updateAccessTokenStatus(token, err, clients)
	}
	slog.Info("HTTP Post was successful, decoding response body",
		slog.String("URL", token.Spec.Url),
		slog.String("namespace", token.Namespace),
		slog.String("name", token.Name))
	return handleTokenResponse(body, token, site, clients)
}

func tlsConfig(token *skupperv2alpha1.AccessToken) *tls.Config {
	if token.Spec.Ca == "" {
		return nil
	}
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM([]byte(token.Spec.Ca))
	return &tls.Config{
		RootCAs: caPool,
	}
}

func postTokenRequest(token *skupperv2alpha1.AccessToken, site *skupperv2alpha1.Site, transport http.RoundTripper) (io.Reader, error) {
	client := &http.Client{
		Transport: transport,
	}
	request, err := http.NewRequest(http.MethodPost, token.Spec.Url, bytes.NewReader([]byte(token.Spec.Code)))
	if err != nil {
		return nil, fmt.Errorf("Controller got error: %s", err)
	}
	request.Header.Add("name", token.Name)
	request.Header.Add("subject", string(site.ObjectMeta.UID))
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Controller got error: %s", err)
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("Controller got failed response: %d (%s) %s", response.StatusCode, http.StatusText(response.StatusCode), strings.TrimSpace(string(body)))
	}
	return response.Body, nil
}

func handleTokenResponse(body io.Reader, token *skupperv2alpha1.AccessToken, site *skupperv2alpha1.Site, clients internalclient.Clients) error {
	decoder := newLinkDecoder(body)
	if err := decoder.decodeAll(); err != nil {
		slog.Error("Could not decode response for AccessToken",
			slog.String("namespace", token.Namespace),
			slog.String("name", token.Name),
			slog.Any("error", err))
		return updateAccessTokenStatus(token, errors.New("Controller could not decode response"), clients)
	}
	refs := []metav1.OwnerReference{
		{
			Kind:       "Site",
			APIVersion: "skupper.io/v2alpha1",
			Name:       site.Name,
			UID:        site.ObjectMeta.UID,
		},
	}
	decoder.secret.ObjectMeta.OwnerReferences = refs
	if _, err := clients.GetKubeClient().CoreV1().Secrets(token.ObjectMeta.Namespace).Create(context.TODO(), &decoder.secret, metav1.CreateOptions{}); err != nil {
		return updateAccessTokenStatus(token, fmt.Errorf("Controller could not create received secret: %s", err), clients)
	}
	for _, link := range decoder.links {
		link.ObjectMeta.OwnerReferences = refs
		if token.Spec.LinkCost > 0 {
			link.Spec.Cost = token.Spec.LinkCost
		}
		if _, err := clients.GetSkupperClient().SkupperV2alpha1().Links(token.ObjectMeta.Namespace).Create(context.TODO(), &link, metav1.CreateOptions{}); err != nil {
			return updateAccessTokenStatus(token, fmt.Errorf("Controller could not create received link: %s", err), clients)
		}
	}

	return updateAccessTokenStatus(token, nil, clients)
}

func updateAccessTokenStatus(token *skupperv2alpha1.AccessToken, err error, clients internalclient.Clients) error {
	if token.SetRedeemed(err) {
		_, err = clients.GetSkupperClient().SkupperV2alpha1().AccessTokens(token.ObjectMeta.Namespace).UpdateStatus(context.TODO(), token, metav1.UpdateOptions{})
		return err
	}
	return nil
}

type LinkDecoder struct {
	decoder *yaml.YAMLOrJSONDecoder
	secret  corev1.Secret
	links   []skupperv2alpha1.Link
}

func newLinkDecoder(r io.Reader) *LinkDecoder {
	return &LinkDecoder{
		decoder: yaml.NewYAMLOrJSONDecoder(r, 1024),
	}
}

func (d *LinkDecoder) decodeSecret() error {
	return d.decoder.Decode(&d.secret)
}

func (d *LinkDecoder) decodeLink() error {
	var link skupperv2alpha1.Link
	if err := d.decoder.Decode(&link); err != nil {
		return err
	}
	d.links = append(d.links, link)
	return nil
}

func (d *LinkDecoder) decodeAll() error {
	if err := d.decodeSecret(); err != nil {
		return err
	}
	for err := d.decodeLink(); err == nil; err = d.decodeLink() {
	}
	return nil
}
