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

func RedeemClaim(claim *skupperv1alpha1.Claim, site *skupperv1alpha1.Site, clients kube.Clients) error {
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
	request, err := http.NewRequest(http.MethodPost, claim.Spec.Url, bytes.NewReader([]byte(claim.Spec.Secret)))
	request.Header.Add("name", claim.Name)
	request.Header.Add("subject", string(site.ObjectMeta.UID))
	response, err := client.Do(request)
	if err != nil {
		return updateClaimStatus(claim, err, clients)
	}
	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err == nil {
			err = fmt.Errorf("Received HTTP Response %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		} else {
			err = fmt.Errorf("Received HTTP Response %d. Could not read body: %s", response.StatusCode, err)
		}
		return updateClaimStatus(claim, err, clients)
	}
	log.Printf("HTTP Post to %s for %s/%s was sucessful, decoding response body", claim.Spec.Url, claim.Namespace, claim.Name)

	decoder := yaml.NewYAMLOrJSONDecoder(response.Body, 1024)
	var link skupperv1alpha1.Link
	if err := decoder.Decode(&link); err != nil {
		return updateClaimStatus(claim, err, clients)
	}
	var secret corev1.Secret
	if err := decoder.Decode(&secret); err != nil {
		return updateClaimStatus(claim, err, clients)
	}

	refs := []metav1.OwnerReference{
		{
			Kind:       "Site",
			APIVersion: "skupper.io/v1alpha1",
			Name:       site.Name,
			UID:        site.ObjectMeta.UID,
		},
	}
	secret.ObjectMeta.OwnerReferences = refs
	link.ObjectMeta.OwnerReferences = refs

	if _, err := clients.GetKubeClient().CoreV1().Secrets(claim.ObjectMeta.Namespace).Create(context.TODO(), &secret, metav1.CreateOptions{}); err != nil {
		return err
	}
	if _, err := clients.GetSkupperClient().SkupperV1alpha1().Links(claim.ObjectMeta.Namespace).Create(context.TODO(), &link, metav1.CreateOptions{}); err != nil {
		return err
	}

	return updateClaimStatus(claim, nil, clients)
}

func updateClaimStatus(claim *skupperv1alpha1.Claim, err error, clients kube.Clients) error {
	if err == nil {
		log.Printf("Redeemed claim %s/%s successfully", claim.Namespace, claim.Name)
		claim.Status.Status = "Ok"
		claim.Status.Claimed = true
	} else {
		log.Printf("Error processing claim %s/%s: %s", claim.Namespace, claim.Name, err)
		claim.Status.Status = err.Error()
	}
	_, err = clients.GetSkupperClient().SkupperV1alpha1().Claims(claim.ObjectMeta.Namespace).UpdateStatus(context.TODO(), claim, metav1.UpdateOptions{})
	return err
}
