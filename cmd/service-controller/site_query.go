package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/skupperproject/skupper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

const (
	SiteQueryError      string = "SiteQueryError"
	SiteQueryRequest    string = "SiteQueryRequest"
	GatewayQueryError   string = "GatewayQueryError"
	GatewayQueryRequest string = "GatewayQueryRequest"
	ServiceCheckError   string = "ServiceCheckError"
	ServiceCheckRequest string = "ServiceCheckRequest"
)

type SiteQueryServer struct {
	client    *client.VanClient
	tlsConfig *tls.Config
	agentPool *qdr.AgentPool
	server    *qdr.RequestServer
	iplookup  *IpLookup
	siteInfo  data.Site
}

func newSiteQueryServer(cli *client.VanClient, config *tls.Config) *SiteQueryServer {
	sqs := SiteQueryServer{
		client:    cli,
		tlsConfig: config,
		agentPool: qdr.NewAgentPool("amqps://"+types.QualifiedServiceName(types.LocalTransportServiceName, cli.Namespace)+":5671", config),
		iplookup:  NewIpLookup(cli),
	}
	sqs.getLocalSiteInfo()
	sqs.server = qdr.NewRequestServer(getSiteQueryAddress(sqs.siteInfo.SiteId), &sqs, sqs.agentPool)
	return &sqs
}

func siteQueryError(err error) {
}

func (s *SiteQueryServer) getLocalSiteInfo() {
	s.siteInfo.SiteId = os.Getenv("SKUPPER_SITE_ID")
	s.siteInfo.SiteName = os.Getenv("SKUPPER_SITE_NAME")
	s.siteInfo.Namespace = os.Getenv("SKUPPER_NAMESPACE")
	s.siteInfo.Version = client.Version
	url, err := getSiteUrl(s.client)
	if err != nil {
		event.Recordf(SiteQueryError, "Failed to get site url: %s", err)
	} else {
		s.siteInfo.Url = url
	}
}

func (s *SiteQueryServer) getLocalSiteQueryData() (data.SiteQueryData, error) {
	data := data.SiteQueryData{
		Site: s.siteInfo,
	}
	agent, err := s.agentPool.Get()
	if err != nil {
		return data, fmt.Errorf("Could not get management agent: %s", err)
	}
	defer s.agentPool.Put(agent)

	routers, err := agent.GetAllRouters()
	if err != nil {
		return data, fmt.Errorf("Error retrieving routers: %s", err)
	}
	err = getServiceInfo(agent, routers, &data, s.iplookup)
	if err != nil {
		return data, fmt.Errorf("Error getting local service info: %s", err)
	}
	return data, nil
}

func (s *SiteQueryServer) getGatewayQueryData() ([]data.SiteQueryData, error) {
	results := []data.SiteQueryData{}
	agent, err := s.agentPool.Get()
	if err != nil {
		return results, fmt.Errorf("Could not get management agent: %s", err)
	}
	defer s.agentPool.Put(agent)

	gateways, err := agent.GetLocalGateways()
	if err != nil {
		return results, fmt.Errorf("Error retrieving gateways: %s", err)
	}
	for _, gateway := range gateways {
		data := data.SiteQueryData{
			Site: data.Site{
				SiteName:  qdr.GetSiteNameForGateway(&gateway),
				SiteId:    gateway.Site.Id,
				Version:   gateway.Site.Version,
				Connected: []string{s.siteInfo.SiteId},
				Edge:      true,
				Gateway:   true,
			},
		}
		err = getServiceInfoForRouters(agent, []qdr.Router{gateway}, &data, s.iplookup)
		if err != nil {
			return results, fmt.Errorf("Error getting local service info: %s", err)
		}
		results = append(results, data)
	}
	return results, nil
}

func getSiteUrl(vanClient *client.VanClient) (string, error) {
	if vanClient.RouteClient == nil {
		service, err := vanClient.KubeClient.CoreV1().Services(vanClient.Namespace).Get(types.TransportServiceName, metav1.GetOptions{})
		if err != nil {
			return "", err
		} else {
			if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
				host := kube.GetLoadBalancerHostOrIp(service)
				return host, nil
			} else {
				return "", nil
			}
		}
	} else {
		route, err := vanClient.RouteClient.Routes(vanClient.Namespace).Get("skupper-inter-router", metav1.GetOptions{})
		if err != nil {
			return "", err
		} else {
			return route.Spec.Host, nil
		}
	}
}

func getSiteQueryAddress(siteId string) string {
	return siteId + "/skupper-site-query"
}

const (
	ServiceCheck string = "service-check"
	GatewayQuery string = "gateway-query"
)

func (s *SiteQueryServer) Request(request *qdr.Request) (*qdr.Response, error) {
	if request.Type == ServiceCheck {
		return s.HandleServiceCheck(request)
	} else if request.Type == GatewayQuery {
		return s.HandleGatewayQuery(request)
	} else {
		return s.HandleSiteQuery(request)
	}
}

func (s *SiteQueryServer) HandleSiteQuery(request *qdr.Request) (*qdr.Response, error) {
	//if request has explicit version, send SiteQueryData, else send LegacySiteData
	if request.Version == "" {
		event.Record(SiteQueryRequest, "legacy site data request")
		data := s.siteInfo.AsLegacySiteInfo()
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("Could not encode response: %s", err)
		}
		return &qdr.Response{
			Version: client.Version,
			Body:    string(bytes),
		}, nil
	} else {
		event.Record(SiteQueryRequest, "site data request")
		data, err := s.getLocalSiteQueryData()
		if err != nil {
			return nil, fmt.Errorf("Could not get response: %s", err)
		}
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("Could not encode response: %s", err)
		}
		return &qdr.Response{
			Version: client.Version,
			Body:    string(bytes),
		}, nil
	}
}

func (s *SiteQueryServer) HandleServiceCheck(request *qdr.Request) (*qdr.Response, error) {
	event.Recordf(ServiceCheckRequest, "checking service %s", request.Body)
	data, err := s.getServiceDetail(context.Background(), request.Body)
	if err != nil {
		return &qdr.Response{
			Version: client.Version,
			Type:    ServiceCheckError,
			Body:    err.Error(),
		}, nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Could not encode service check response: %s", err)
	}
	return &qdr.Response{
		Version: client.Version,
		Type:    request.Type,
		Body:    string(bytes),
	}, nil
}

func (s *SiteQueryServer) HandleGatewayQuery(request *qdr.Request) (*qdr.Response, error) {
	event.Record(GatewayQueryRequest, "gateway request")
	data, err := s.getGatewayQueryData()
	if err != nil {
		return nil, fmt.Errorf("Could not get response: %s", err)
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Could not encode response: %s", err)
	}
	return &qdr.Response{
		Version: client.Version,
		Body:    string(bytes),
	}, nil

}

func (s *SiteQueryServer) run() {
	for {
		ctxt := context.Background()
		err := s.server.Run(ctxt)
		if err != nil {
			event.Recordf(SiteQueryError, "Error handling requests: %s", err)
		}
	}
}

func (s *SiteQueryServer) start(stopCh <-chan struct{}) error {
	err := s.iplookup.start(stopCh)
	go s.run()
	return err
}

func getTcpAddressFilter(address string) qdr.TcpEndpointFilter {
	return func(endpoint *qdr.TcpEndpoint) bool {
		return matchQualifiedAddress(address, endpoint.Address)
	}
}

func getHttpAddressFilter(address string) qdr.HttpEndpointFilter {
	return func(endpoint *qdr.HttpEndpoint) bool {
		return matchQualifiedAddress(address, endpoint.Address)
	}
}

func matchQualifiedAddress(unqualified string, qualified string) bool {
	return unqualified == strings.Split(qualified, ":")[0]
}

func (s *SiteQueryServer) getServiceDetail(context context.Context, address string) (data.ServiceDetail, error) {
	detail := data.ServiceDetail{
		SiteId: s.siteInfo.SiteId,
	}
	definition, err := s.client.ServiceInterfaceInspect(context, address)
	if err != nil {
		return detail, err
	}
	if definition == nil {
		return detail, fmt.Errorf("No such service %q", address)
	}
	detail.Definition = *definition

	service, err := kube.GetService(address, s.client.Namespace, s.client.KubeClient)
	if err != nil {
		return detail, err
	}

	detail.IngressBinding.ServicePorts = map[int]int{}
	for _, ports := range service.Spec.Ports {
		if utils.IntSliceContains(detail.Definition.Ports, int(ports.Port)) {
			detail.IngressBinding.ServicePorts[int(ports.Port)] = ports.TargetPort.IntValue()
		} else {
			detail.AddObservation(fmt.Sprintf("Kubernetes service defines port %s %d:%d which is not in skupper service definition", ports.Name, ports.Port, ports.TargetPort.IntValue()))
		}
	}

	detail.IngressBinding.ServiceSelector = service.Spec.Selector

	agent, err := s.agentPool.Get()
	if err != nil {
		return detail, fmt.Errorf("Could not get management agent: %s", err)
	}
	defer s.agentPool.Put(agent)

	if detail.Definition.Protocol == "tcp" {
		listeners, err := agent.GetLocalTcpListeners(getTcpAddressFilter(detail.Definition.Address))
		if err != nil {
			detail.AddObservation(fmt.Sprintf("Error retrieving tcp listeners: %s", err))
		} else {
			detail.ExtractTcpListenerPorts(listeners)
		}

		connectors, err := agent.GetLocalTcpConnectors(getTcpAddressFilter(detail.Definition.Address))
		if err != nil {
			detail.AddObservation(fmt.Sprintf("Error retrieving tcp connectors for %s: %s", detail.Definition.Address, err))
		} else {
			detail.ExtractTcpConnectorPorts(connectors)
		}
	} else if detail.Definition.Protocol == "http" || detail.Definition.Protocol == "http2" {
		listeners, err := agent.GetLocalHttpListeners(getHttpAddressFilter(detail.Definition.Address))
		if err != nil {
			detail.AddObservation(fmt.Sprintf("Error retrieving http listeners: %s", err))
		} else {
			detail.ExtractHttpListenerPorts(listeners)
		}

		connectors, err := agent.GetLocalHttpConnectors(getHttpAddressFilter(detail.Definition.Address))
		if err != nil {
			detail.AddObservation(fmt.Sprintf("Error retrieving http connectors for %s: %s", detail.Definition.Address, err))
		} else {
			detail.ExtractHttpConnectorPorts(connectors)
		}
	} else {
		detail.AddObservation(fmt.Sprintf("Unrecognised protocol: %s", detail.Definition.Protocol))
	}

	if len(detail.Definition.Targets) > 0 && len(detail.EgressBindings) == 0 {
		detail.AddObservation(fmt.Sprintf("No connectors on %s for %s ", detail.SiteId, detail.Definition.Address))
	}
	return detail, nil
}

func querySites(agent qdr.RequestResponse, sites []data.SiteQueryData) {
	for i, s := range sites {
		request := qdr.Request{
			Address: getSiteQueryAddress(s.SiteId),
			Version: client.Version,
		}
		response, err := agent.Request(&request)
		if err != nil {
			event.Recordf(SiteQueryError, "Request to %s failed: %s", s.SiteId, err)
		} else if response.Version == "" {
			//assume legacy version of site-query protocol
			info := data.LegacySiteInfo{}
			err := json.Unmarshal([]byte(response.Body), &info)
			if err != nil {
				event.Recordf(SiteQueryError, "Error parsing legacy json %q from %s: %s", response.Body, s.SiteId, err)
			} else {
				sites[i].SiteName = info.SiteName
				sites[i].Namespace = info.Namespace
				sites[i].Url = info.Url
				sites[i].Version = info.Version
			}
		} else {
			site := data.SiteQueryData{}
			err := json.Unmarshal([]byte(response.Body), &site)
			if err != nil {
				event.Recordf(SiteQueryError, "Error parsing json for site query %q from %s: %s", response.Body, s.SiteId, err)
			} else {
				sites[i].SiteName = site.SiteName
				sites[i].Namespace = site.Namespace
				sites[i].Url = site.Url
				sites[i].Version = site.Version
				sites[i].TcpServices = site.TcpServices
				sites[i].HttpServices = site.HttpServices
			}
		}
	}
}

func queryGateways(agent qdr.RequestResponse, sites []data.SiteQueryData) []data.SiteQueryData {
	gateways := []data.SiteQueryData{}
	for _, s := range sites {
		request := qdr.Request{
			Address: getSiteQueryAddress(s.SiteId),
			Version: client.Version,
			Type:    GatewayQuery,
		}
		response, err := agent.Request(&request)
		if err != nil {
			event.Recordf(GatewayQueryError, "Request to %s failed: %s", s.SiteId, err)
		} else {
			sites := []data.SiteQueryData{}
			err := json.Unmarshal([]byte(response.Body), &sites)
			if err != nil {
				event.Recordf(SiteQueryError, "Error parsing json for site query %q from %s: %s", response.Body, s.SiteId, err)
			}
			gateways = append(gateways, sites...)
		}
	}
	return gateways
}

func getServiceInfo(agent *qdr.Agent, network []qdr.Router, site *data.SiteQueryData, lookup data.NameMapping) error {
	return getServiceInfoForRouters(agent, qdr.GetRoutersForSite(network, site.SiteId), site, lookup)
}

func getServiceInfoForRouters(agent *qdr.Agent, routers []qdr.Router, site *data.SiteQueryData, lookup data.NameMapping) error {
	bridges, err := agent.GetBridges(routers)
	if err != nil {
		return fmt.Errorf("Error retrieving bridge configuration: %s", err)
	}
	httpRequestInfo, err := agent.GetHttpRequestInfo(routers)
	if err != nil {
		return fmt.Errorf("Error retrieving http request info: %s", err)
	}
	tcpConnections, err := agent.GetTcpConnections(routers)
	if err != nil {
		return fmt.Errorf("Error retrieving tcp connection info: %s", err)
	}
	site.HttpServices = data.GetHttpServices(site.SiteId, httpRequestInfo, qdr.GetHttpConnectors(bridges), qdr.GetHttpListeners(bridges), lookup)
	site.TcpServices = data.GetTcpServices(site.SiteId, tcpConnections, qdr.GetTcpConnectors(bridges), lookup)
	return nil
}

func checkServiceForSites(agent qdr.RequestResponse, address string, sites *data.ServiceCheck) error {
	details := []data.ServiceDetail{}
	for _, s := range sites.Details {
		request := qdr.Request{
			Address: getSiteQueryAddress(s.SiteId),
			Version: client.Version,
			Type:    ServiceCheck,
			Body:    address,
		}
		response, err := agent.Request(&request)
		if err != nil {
			event.Recordf(ServiceCheckError, "Request to %s failed: %s", s.SiteId, err)
			return err
		}
		if response.Type == ServiceCheckError {
			sites.AddObservation(fmt.Sprintf("%s on %s", response.Body, s.SiteId))
		} else {
			detail := data.ServiceDetail{}
			err = json.Unmarshal([]byte(response.Body), &detail)
			if err != nil {
				event.Recordf(ServiceCheckError, "Error parsing json for service check %q from %s: %s", response.Body, s.SiteId, err)
				return err
			}
			details = append(details, detail)
		}
	}
	sites.Details = details
	data.CheckService(sites)
	return nil
}
