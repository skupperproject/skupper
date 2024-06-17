package claims

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
)

func RedeemAccessToken(claim *skupperv1alpha1.AccessToken, site *skupperv1alpha1.Site, clients kube.Clients) error {
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
	}
	request, err := http.NewRequest(http.MethodPost, claim.Spec.Url, bytes.NewReader([]byte(claim.Spec.Code)))
	request.Header.Add("name", claim.Name)
	request.Header.Add("subject", string(site.ObjectMeta.UID))
	response, err := client.Do(request)
	if err != nil {
		return updateAccessTokenStatus(claim, err, clients)
	}
	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err == nil {
			err = fmt.Errorf("Received HTTP Response %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		} else {
			err = fmt.Errorf("Received HTTP Response %d. Could not read body: %s", response.StatusCode, err)
		}
		return updateAccessTokenStatus(claim, err, clients)
	}
	log.Printf("HTTP Post to %s for %s/%s was sucessful, decoding response body", claim.Spec.Url, claim.Namespace, claim.Name)

	decoder := newLinkDecoder(response.Body)
	if err := decoder.decodeAll(); err != nil {
		return updateAccessTokenStatus(claim, err, clients)
	}
	refs := []metav1.OwnerReference{
		{
			Kind:       "Site",
			APIVersion: "skupper.io/v1alpha1",
			Name:       site.Name,
			UID:        site.ObjectMeta.UID,
		},
	}
	decoder.secret.ObjectMeta.OwnerReferences = refs
	if _, err := clients.GetKubeClient().CoreV1().Secrets(claim.ObjectMeta.Namespace).Create(context.TODO(), &decoder.secret, metav1.CreateOptions{}); err != nil {
		return err
	}
	for _, link := range decoder.links {
		link.ObjectMeta.OwnerReferences = refs
		if _, err := clients.GetSkupperClient().SkupperV1alpha1().Links(claim.ObjectMeta.Namespace).Create(context.TODO(), &link, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return updateAccessTokenStatus(claim, nil, clients)
}

func updateAccessTokenStatus(claim *skupperv1alpha1.AccessToken, err error, clients kube.Clients) error {
	if err == nil {
		log.Printf("Redeemed claim %s/%s successfully", claim.Namespace, claim.Name)
		claim.Status.Status = "Ok"
		claim.Status.Redeemed = true
	} else {
		log.Printf("Error processing claim %s/%s: %s", claim.Namespace, claim.Name, err)
		claim.Status.Status = err.Error()
	}
	_, err = clients.GetSkupperClient().SkupperV1alpha1().AccessTokens(claim.ObjectMeta.Namespace).UpdateStatus(context.TODO(), claim, metav1.UpdateOptions{})
	return err
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
