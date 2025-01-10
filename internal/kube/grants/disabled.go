package grants

import (
	"context"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type GrantsDisabled struct {
	clients internalclient.Clients
}

func (s *GrantsDisabled) markGrantNotEnabled(key string, grant *skupperv2alpha1.AccessGrant) error {
	if grant == nil || !grant.Status.SetStatusMessage("AccessGrants are not enabled") {
		return nil
	}
	if _, err := s.clients.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{}); err != nil {
		log.Printf("AccessGrants are not enabled. Error updating status for %s: %s", key, err)
	}
	return nil
}

func disabled(controller *internalclient.Controller, watchNamespace string) *GrantsDisabled {
	mgr := &GrantsDisabled{
		clients: controller,
	}
	controller.WatchAccessGrants(watchNamespace, mgr.markGrantNotEnabled)
	return mgr
}
