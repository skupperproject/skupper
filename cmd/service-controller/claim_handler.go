package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/event"
	corev1 "k8s.io/api/core/v1"
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
	handler.redeemer = domain.NewClaimRedeemer(handler.name, site.Id, site.Version, handler.updateSecret, event.Recordf)
	return NewSecretController(handler.name, types.ClaimRequestSelector, cli.KubeClient, cli.Namespace, handler)
}

func (h *ClaimHandler) updateSecret(claim *corev1.Secret) error {
	_, err := h.vanClient.KubeClient.CoreV1().Secrets(h.vanClient.Namespace).Update(claim)
	return err
}
