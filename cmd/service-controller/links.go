package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gorilla/mux"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/qdr"
)

const (
	LinkManagement string = "LinkManagement"
)

type Links interface {
	getLinks() ([]types.LinkStatus, error)
	getLink(name string) (*types.LinkStatus, error)
	deleteLink(name string) (bool, error)
	createLink(cost int, token []byte) error
}

type Connectors interface {
	getConnectorStatus() (map[string]qdr.ConnectorStatus, error)
}

type ConnectorManager struct {
	agentPool *qdr.AgentPool
}

func newConnectorManager(pool *qdr.AgentPool) *ConnectorManager {
	return &ConnectorManager{
		agentPool: pool,
	}
}

func (m *ConnectorManager) getConnectorStatus() (map[string]qdr.ConnectorStatus, error) {
	agent, err := m.agentPool.Get()
	if err != nil {
		return map[string]qdr.ConnectorStatus{}, fmt.Errorf("Could not get management agent: %s", err)
	}
	defer m.agentPool.Put(agent)
	return agent.GetLocalConnectorStatus()
}

type LinkManager struct {
	cli        *client.VanClient
	agentPool  *qdr.AgentPool
	connectors Connectors
}

func newLinkManager(cli *client.VanClient, pool *qdr.AgentPool) *LinkManager {
	return &LinkManager{
		cli:        cli,
		connectors: newConnectorManager(pool),
	}
}

func isTokenOrClaim(s *corev1.Secret) (bool, bool) {
	if s.ObjectMeta.Labels != nil {
		if typename, ok := s.ObjectMeta.Labels[types.SkupperTypeQualifier]; ok {
			return typename == types.TypeToken, typename == types.TypeClaimRequest
		}
	}
	return false, false
}

func getLinkFromClaim(s *corev1.Secret) *types.LinkStatus {
	link := types.LinkStatus{
		Name:       s.ObjectMeta.Name,
		Configured: false,
		Created:    s.ObjectMeta.CreationTimestamp.Format(time.RFC3339),
	}
	if s.ObjectMeta.Annotations != nil {
		link.Url = s.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey]
		if desc, ok := s.ObjectMeta.Annotations[types.StatusAnnotationKey]; ok {
			link.Description = "Failed to redeem claim: " + desc
		}
		if value, ok := s.ObjectMeta.Annotations[types.TokenCost]; ok {
			cost, err := strconv.Atoi(value)
			if err == nil {
				link.Cost = cost
			}
		}
	}
	return &link
}

func getLinkFromToken(s *corev1.Secret, connectors map[string]qdr.ConnectorStatus) *types.LinkStatus {
	link := types.LinkStatus{
		Name:       s.ObjectMeta.Name,
		Configured: true,
		Created:    s.ObjectMeta.CreationTimestamp.Format(time.RFC3339),
	}
	if status, ok := connectors[link.Name]; ok {
		link.Url = fmt.Sprintf("%s:%s", status.Host, status.Port)
		link.Cost = status.Cost
		link.Connected = status.Status == "SUCCESS"
		link.Description = status.Description
	}
	return &link
}

func getLinkStatus(secret *corev1.Secret, connectors map[string]qdr.ConnectorStatus) *types.LinkStatus {
	isToken, isClaim := isTokenOrClaim(secret)
	if isClaim {
		return getLinkFromClaim(secret)
	} else if isToken {
		return getLinkFromToken(secret, connectors)
	} else {
		return nil
	}
}

func (m *LinkManager) getLinks() ([]types.LinkStatus, error) {
	links := []types.LinkStatus{}
	secrets, err := m.cli.KubeClient.CoreV1().Secrets(m.cli.Namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type in (connection-token, token-claim)"})
	if err != nil {
		return links, err
	}
	connectors, err := m.connectors.getConnectorStatus()
	if err != nil {
		event.Recordf(LinkManagement, "Failed to retrieve connector status: %s", err)
	}
	for _, secret := range secrets.Items {
		link := getLinkStatus(&secret, connectors)
		if link != nil {
			links = append(links, *link)
		}
	}
	return links, nil
}

func (m *LinkManager) getLink(name string) (*types.LinkStatus, error) {
	secret, err := m.cli.KubeClient.CoreV1().Secrets(m.cli.Namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	connectors, err := m.connectors.getConnectorStatus()
	if err != nil {
		return nil, err
	}
	return getLinkStatus(secret, connectors), nil
}

func (m *LinkManager) deleteLink(name string) (bool, error) {
	err := m.cli.ConnectorRemove(context.Background(), types.ConnectorRemoveOptions{Name: name, SkupperNamespace: m.cli.Namespace})
	if err != nil {
		return false, err
	}
	event.Recordf(LinkManagement, "Deleted link %q", name)
	return true, nil
}

func (m *LinkManager) createLink(cost int, token []byte) error {
	secret, err := m.cli.ConnectorCreateSecretFromData(context.Background(), types.ConnectorCreateOptions{Cost: int32(cost), SkupperNamespace: m.cli.Namespace, Yaml: token})
	if err != nil {
		return err
	}
	event.Recordf(LinkManagement, "Created link %q", secret.ObjectMeta.Name)
	return nil
}

func getCost(r *http.Request) (int, error) {
	params := r.URL.Query()
	value := params.Get("cost")
	if value == "" {
		return 0, nil
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return result, nil
}

func writeJson(obj interface{}, w http.ResponseWriter) {
	bytes, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		fmt.Fprintf(w, string(bytes)+"\n")
	}
}

func serveLinks(m Links) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if r.Method == http.MethodGet {
			if name, ok := vars["name"]; ok {
				link, err := m.getLink(name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else if link == nil {
					http.Error(w, "No such link", http.StatusNotFound)
				} else {
					writeJson(link, w)
				}

			} else {
				links, err := m.getLinks()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else {
					writeJson(links, w)
				}
			}
		} else if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			cost, err := getCost(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			err = m.createLink(cost, body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else if r.Method == http.MethodDelete {
			if name, ok := vars["name"]; ok {
				ok, err := m.deleteLink(name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else if !ok {
					http.Error(w, "No such link", http.StatusNotFound)
				} else {
					event.Recordf("Link %s deleted", name)
				}
			} else {
				http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			}
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})
}
