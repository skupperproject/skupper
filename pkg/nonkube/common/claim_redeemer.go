package common

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/nonkube/api"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func RedeemClaims(siteState *api.SiteState) error {
	var errs []error

	for name, claim := range siteState.Claims {
		err := redeemAccessToken(claim, siteState)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to redeem claim %s: %w", name, err))
			fmt.Printf("RedeemClaims: failed to redeem claim: %s: %s\n", name, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to redeem %d claims", len(errs))
	}

	return nil
}

// Redeem logic that populates siteState.Secrets and siteState.Links
func redeemAccessToken(claim *skupperv1alpha1.AccessToken, siteState *api.SiteState) error {
	transport := &http.Transport{}
	if claim.Spec.Ca != "" {
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM([]byte(claim.Spec.Ca))
		transport.TLSClientConfig = &tls.Config{
			RootCAs: caPool,
		}
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	request, err := http.NewRequest(http.MethodPost, claim.Spec.Url, bytes.NewReader([]byte(claim.Spec.Code)))
	if err != nil {
		return err
	}
	request.Header.Add("name", claim.Name)
	request.Header.Add("subject", string(siteState.Site.Name))
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err == nil {
			err = fmt.Errorf("Received HTTP Response %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		} else {
			err = fmt.Errorf("Received HTTP Response %d. Could not read body: %s", response.StatusCode, err)
		}
		return err
	}
	// TODO should bootstrap log helpful status info (like the following)?
	// log.Printf("HTTP Post to %s for %s/%s was sucessful, decoding response body", claim.Spec.Url, claim.Namespace, claim.Name)

	decoder := newLinkDecoder(response.Body)
	if err := decoder.decodeAll(); err != nil {
		return err
	}

	siteState.Secrets[decoder.secret.ObjectMeta.Name] = &decoder.secret

	for _, link := range decoder.links {
		siteState.Links[link.ObjectMeta.Name] = &link
	}

	return nil
}

type LinkDecoder struct {
	decoder *yaml.YAMLOrJSONDecoder
	secret  corev1.Secret
	links   []skupperv1alpha1.Link
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
	var link skupperv1alpha1.Link
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
	for err := d.decodeLink(); err != io.EOF; err = d.decodeLink() {
		if err != nil {
			return err
		}
	}
	return nil
}
