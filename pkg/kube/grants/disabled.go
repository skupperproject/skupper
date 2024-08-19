package grants

import (
	"context"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
)

type GrantsDisabled struct {
	clients internalclient.Clients
}

func (s *GrantsDisabled) markGrantNotEnabled(key string, grant *skupperv1alpha1.AccessGrant) error {
	if grant == nil || !grant.Status.SetStatusMessage("AccessGrants are not enabled") {
		return nil
	}
	if _, err := s.clients.GetSkupperClient().SkupperV1alpha1().AccessGrants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{}); err != nil {
		log.Printf("AccessGrants are not enabled. Error updating status for %s: %s", key, err)
	}
	return nil
}

func disabled(controller *kube.Controller, watchNamespace string) *GrantsDisabled {
	mgr := &GrantsDisabled{
		clients: controller,
	}
	controller.WatchAccessGrants(watchNamespace, mgr.markGrantNotEnabled)
	return mgr
}
