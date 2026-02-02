package grants

import (
	"context"
	"log/slog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type GrantsDisabled struct {
	clients internalclient.Clients
	logger  *slog.Logger
}

func (s *GrantsDisabled) markGrantNotEnabled(key string, grant *skupperv2alpha1.AccessGrant) error {
	if grant == nil || !grant.Status.SetStatusMessage("AccessGrants are not enabled") {
		return nil
	}
	if _, err := s.clients.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.ObjectMeta.Namespace).UpdateStatus(context.TODO(), grant, metav1.UpdateOptions{}); err != nil {
		s.logger.Error("AccessGrants are not enabled. Error updating status", slog.String("key", key), slog.Any("error", err))
	}
	return nil
}

func disabled(eventProcessor *watchers.EventProcessor, watchNamespace string) *GrantsDisabled {
	mgr := &GrantsDisabled{
		clients: eventProcessor,
		logger:  slog.New(slog.Default().Handler()).With(slog.String("component", "kube.grants.disabled")),
	}
	eventProcessor.WatchAccessGrants(watchNamespace, mgr.markGrantNotEnabled)
	return mgr
}
