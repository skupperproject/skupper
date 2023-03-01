package main

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type ClaimHandler struct {
	name      string
	vanClient *client.VanClient
	siteId    string
	redeemer  *domain.ClaimRedeemer
}

func (h *ClaimHandler) Handle(name string, claim *corev1.Secret) error {
	if claim != nil {
		return h.redeemer.RedeemClaim(claim)
	}
	return nil
}

func newClaimHandler(cli *client.VanClient, siteId string) *SecretController {
	handler := &ClaimHandler{
		name:      "ClaimHandler",
		vanClient: cli,
		siteId:    siteId,
	}
	site, _ := cli.GetSiteMetadata()
	if site == nil {
		site = &qdr.SiteMetadata{}
	}
	handler.redeemer = domain.NewClaimRedeemer(handler.name, site.Id, site.Version, handler.updateSecret, event.Recordf)
	return NewSecretController(handler.name, types.ClaimRequestSelector, cli.KubeClient, cli.Namespace, handler)
}

func (h *ClaimHandler) updateSecret(claim *corev1.Secret) error {
	_, err := h.vanClient.KubeClient.CoreV1().Secrets(h.vanClient.Namespace).Update(context.TODO(), claim, metav1.UpdateOptions{})
	return err
}
