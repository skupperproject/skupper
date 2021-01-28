package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type SiteQueryServer struct {
	tlsConfig *tls.Config
	agentPool *qdr.AgentPool
	server    *qdr.RequestServer
	iplookup  *IpLookup
	siteInfo  data.Site
}

func newSiteQueryServer(cli *client.VanClient, config *tls.Config) *SiteQueryServer {
	sqs := SiteQueryServer{
		tlsConfig: config,
		agentPool: qdr.NewAgentPool("amqps://skupper-messaging:5671", config),
		iplookup:  NewIpLookup(cli),
	}
	sqs.getLocalSiteInfo(cli)
	sqs.server = qdr.NewRequestServer(getSiteQueryAddress(sqs.siteInfo.SiteId), &sqs, sqs.agentPool)
	return &sqs
}

func (s *SiteQueryServer) getLocalSiteInfo(vanClient *client.VanClient) {
	s.siteInfo.SiteId = os.Getenv("SKUPPER_SITE_ID")
	s.siteInfo.SiteName = os.Getenv("SKUPPER_SITE_NAME")
	s.siteInfo.Namespace = os.Getenv("SKUPPER_NAMESPACE")
	s.siteInfo.Version = client.Version
	url, err := getSiteUrl(vanClient)
	if err != nil {
		log.Printf("Failed to get site url: %s", err)
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

func getSiteUrl(vanClient *client.VanClient) (string, error) {
	if vanClient.RouteClient == nil {
		service, err := vanClient.KubeClient.CoreV1().Services(vanClient.Namespace).Get("skupper-internal", metav1.GetOptions{})
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

func (s *SiteQueryServer) Request(request *qdr.Request) (*qdr.Response, error) {
	//if request has explicit version, send SiteQueryData, else send LegacySiteData
	if request.Version == "" {
		log.Printf("Sending legacy data for request")
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

func (s *SiteQueryServer) run() {
	for {
		ctxt := context.Background()
		err := s.server.Run(ctxt)
		if err != nil {
			log.Printf("[site-query] Error handling requests: %s", err)
		}
	}
}

func (s *SiteQueryServer) start(stopCh <-chan struct{}) error {
	err := s.iplookup.start(stopCh)
	go s.run()
	return err
}

func querySites(agent qdr.RequestResponse, sites []data.SiteQueryData) {
	for i, s := range sites {
		request := qdr.Request{
			Address: getSiteQueryAddress(s.SiteId),
			Version: client.Version,
		}
		response, err := agent.Request(&request)
		if err != nil {
			log.Printf("[site-query] Request to %s failed: %s", s.SiteId, err)
		} else if response.Version == "" {
			//assume legacy version of site-query protocol
			info := data.LegacySiteInfo{}
			err := json.Unmarshal([]byte(response.Body), &info)
			if err != nil {
				log.Printf("[site-query] Error parsing legacy json %q from %s: %s", response.Body, s.SiteId, err)
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
				log.Printf("Error parsing json for site query %q from %s: %s", response.Body, s.SiteId, err)
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

func getServiceInfo(agent *qdr.Agent, network []qdr.Router, site *data.SiteQueryData, lookup data.NameMapping) error {
	routers := qdr.GetRoutersForSite(network, site.SiteId)
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
