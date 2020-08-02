package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	amqp "github.com/interconnectedcloud/go-amqp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type SiteInfo struct {
	SiteId    string
	SiteName  string
	Namespace string
	Url       string
}

type SiteQueryServer struct {
	tlsConfig *tls.Config
	siteInfo  SiteInfo
}

func newSiteQueryServer(tlsConfig *tls.Config) *SiteQueryServer {
	return &SiteQueryServer{
		tlsConfig: tlsConfig,
	}
}

func (s *SiteQueryServer) getLocalSiteInfo(vanClient *client.VanClient) {
	s.siteInfo.SiteId = os.Getenv("SKUPPER_SITE_ID")
	s.siteInfo.SiteName = os.Getenv("SKUPPER_SITE_NAME")
	s.siteInfo.Namespace = os.Getenv("SKUPPER_NAMESPACE")
	url, err := getSiteUrl(vanClient)
	if err != nil {
		log.Printf("Failed to get site url: %s", err)
	} else {
		s.siteInfo.Url = url
	}
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

func (s *SiteQueryServer) run() {
	ctx := context.Background()

	log.Println("Establishing connection to skupper-messaging service for site query server")

	client, err := amqp.Dial("amqps://skupper-messaging:5671", amqp.ConnSASLExternal(), amqp.ConnMaxFrameSize(4294967295), amqp.ConnTLSConfig(s.tlsConfig))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed to create amqp connection for site query server: %s", err.Error()))
		return
	}
	log.Println("Site query server connection to skupper-messaging service established")
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed to create amqp session %s", err.Error()))
		return
	}

	receiver, err := session.NewReceiver(
		amqp.LinkSourceAddress(getSiteQueryAddress(s.siteInfo.SiteId)),
		amqp.LinkCredit(10),
	)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed to create amqp receiver %s", err.Error()))
		return
	}
	sender, err := session.NewSender()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed to create sender: %s", err))
		return
	}
	for {
		msg, err := receiver.Receive(ctx)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Failed reading message from service sync %s", err.Error()))
			return
		}
		msg.Accept()

		bytes, err := json.Marshal(s.siteInfo)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Could not encode response: %s", err))
			return
		}

		correlationId, ok := qdr.AsUint64(msg.Properties.CorrelationID)
		if !ok {
			log.Printf("WARN: Could not get correlationid from site query request: %#v (%T)", msg.Properties.CorrelationID, msg.Properties.CorrelationID)
		}
		response := amqp.Message{
			Properties: &amqp.MessageProperties{
				To:            msg.Properties.ReplyTo,
				CorrelationID: correlationId,
			},
			Value: string(bytes),
		}

		err = sender.Send(ctx, &response)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Could not send response: %s", err))
			return
		}
	}
}

func getAllSiteInfo(agent *qdr.Agent, sites []Site) error {
	addresses := make([]string, len(sites))
	for i, s := range sites {
		addresses[i] = getSiteQueryAddress(s.SiteId)
	}
	results, err := agent.SiteQuery(addresses)
	if err != nil {
		return err
	}
	errors := []string{}
	for i, r := range results {
		info := SiteInfo{}
		err := json.Unmarshal([]byte(r), &info)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Error parsing json for site query '%s' from %s: %s", r, sites[i].SiteId, err))
		} else {
			sites[i].SiteName = info.SiteName
			sites[i].Namespace = info.Namespace
			sites[i].Url = info.Url
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, ", "))
	}
	return nil
}
