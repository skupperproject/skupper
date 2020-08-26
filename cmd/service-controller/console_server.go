package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type ConsoleServer struct {
	agentPool *qdr.AgentPool
	iplookup  *IpLookup
}

func newConsoleServer(cli *client.VanClient, config *tls.Config) *ConsoleServer {
	return &ConsoleServer{
		agentPool: qdr.NewAgentPool("amqps://skupper-messaging:5671", config),
		iplookup:  NewIpLookup(cli),
	}
}

func authenticate(dir string, user string, password string) bool {
	filename := path.Join(dir, user)
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Failed to authenticate %s, no such user exists", user)
		} else {
			log.Printf("Failed to authenticate %s: %s", user, err)
		}
		return false
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("Failed to authenticate %s: %s", user, err)
		return false
	}
	return string(bytes) == password
}

func authenticated(h http.Handler) http.Handler {
	dir := os.Getenv("METRICS_USERS")
	if dir != "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, password, _ := r.BasicAuth()

			if authenticate(dir, user, password) {
				h.ServeHTTP(w, r)
			} else {
				w.Header().Set("WWW-Authenticate", "Basic realm=skupper")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		})
	} else {
		return h
	}
}

func (server *ConsoleServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	agent, err := server.agentPool.Get()
	if err != nil {
		log.Printf("Could not get management agent : %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	data, err := getConsoleData(agent, server.iplookup)
	server.agentPool.Put(agent)
	if err != nil {
		log.Printf("Error retrieving console data: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		bytes, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			log.Printf("Error writing json: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			fmt.Fprintf(w, string(bytes)+"\n")
		}
	}
}

func (server *ConsoleServer) start(stopCh <-chan struct{}) error {
	err := server.iplookup.start(stopCh)
	go server.listen()
	return err
}

func (server *ConsoleServer) listen() {
	addr := ":8080"
	if os.Getenv("METRICS_PORT") != "" {
		addr = ":" + os.Getenv("METRICS_PORT")
	}
	if os.Getenv("METRICS_HOST") != "" {
		addr = os.Getenv("METRICS_HOST") + addr
	}
	log.Printf("Console server listening on %s", addr)
	http.Handle("/DATA", authenticated(server))
	http.Handle("/", authenticated(http.FileServer(http.Dir("/app/console/"))))
	log.Fatal(http.ListenAndServe(addr, nil))
}

type ServiceStats struct {
	Address  string          `json:"address"`
	Protocol string          `json:"protocol"`
	Targets  []ServiceTarget `json:"targets"`
}

type ServiceTarget struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	SiteId string `json:"site_id"`
}

type HttpRequestsHandledList []HttpRequestsHandled
type HttpRequestsReceivedList []HttpRequestsReceived

type HttpServiceStats struct {
	ServiceStats
	RequestsReceived HttpRequestsReceivedList `json:"requests_received"`
	RequestsHandled  HttpRequestsHandledList  `json:"requests_handled"`
}

type HttpRequestsReceived struct {
	SiteId   string                      `json:"site_id"`
	ByClient map[string]HttpRequestStats `json:"by_client,omitempty"`
}

type HttpRequestsHandled struct {
	SiteId            string                      `json:"site_id"`
	ByServer          map[string]HttpRequestStats `json:"by_server,omitempty"`
	ByOriginatingSite map[string]HttpRequestStats `json:"by_originating_site,omitempty"`
}

type HttpRequestStats struct {
	Requests       int                         `json:"requests"`
	BytesIn        int                         `json:"bytes_in"`
	BytesOut       int                         `json:"bytes_out"`
	Details        map[string]int              `json:"details"`
	LatencyMax     int                         `json:"latency_max"`
	ByHandlingSite map[string]HttpRequestStats `json:"by_handling_site,omitempty"`
}

type TcpServiceStats struct {
	ServiceStats
	ConnectionsIngress SiteConnectionsList `json:"connections_ingress,omitempty"`
	ConnectionsEgress  SiteConnectionsList `json:"connections_egress,omitempty"`
}

type SiteConnectionsList []SiteConnections

type SiteConnections struct {
	SiteId      string                     `json:"site_id"`
	Connections map[string]ConnectionStats `json:"connections"`
}

type ConnectionStats struct {
	Id        string `json:"id"`
	StartTime uint64 `json:"start_time"`
	LastOut   uint64 `json:"last_out"`
	LastIn    uint64 `json:"last_in"`
	BytesIn   int    `json:"bytes_in"`
	BytesOut  int    `json:"bytes_out"`
	Client    string `json:"client,omitempty"`
	Server    string `json:"server,omitempty"`
}

type TcpServiceStatsMap map[string]TcpServiceStats
type HttpServiceStatsMap map[string]HttpServiceStats

func getHttpRequestDetails(in qdr.Record) map[string]int {
	out := map[string]int{}
	for k, v := range in {
		i, ok := qdr.AsInt(v)
		if ok {
			out[k] = i
		}
	}
	return out
}

func max(a int, b int) int {
	if b > a {
		return b
	} else {
		return a
	}
}

func mergeCounts(a map[string]int, b map[string]int) {
	for k, v := range b {
		if s, ok := a[k]; ok {
			a[k] = s + v
		} else {
			a[k] = v
		}
	}
}

func (a *HttpRequestStats) merge(b *HttpRequestStats) {
	a.Requests += b.Requests
	a.BytesIn += b.BytesIn
	a.BytesOut += b.BytesOut
	a.LatencyMax = max(a.LatencyMax, b.LatencyMax)
	mergeCounts(a.Details, b.Details)
	if a.ByHandlingSite == nil {
		a.ByHandlingSite = b.ByHandlingSite
	} else if b.ByHandlingSite != nil {
		mergeHttpRequestStats(a.ByHandlingSite, b.ByHandlingSite)
	}
}

func mergeHttpRequestStats(a map[string]HttpRequestStats, b map[string]HttpRequestStats) {
	for k, v := range b {
		if s, ok := a[k]; ok {
			s.merge(&v)
			a[k] = s
		} else {
			a[k] = v
		}
	}
}

func getHttpRequestStats(in qdr.Record) map[string]HttpRequestStats {
	out := map[string]HttpRequestStats{}
	for k, v := range in {
		m, ok := v.(map[string]interface{})
		if ok {
			r := qdr.Record(m)
			hrs := HttpRequestStats{
				Requests:   r.AsInt("requests"),
				BytesIn:    r.AsInt("bytes_in"),
				BytesOut:   r.AsInt("bytes_out"),
				Details:    getHttpRequestDetails(r.AsRecord("details")),
				LatencyMax: r.AsInt("latency_max"),
			}
			byHandlingSite := r.AsRecord("by_handling_site")
			if byHandlingSite != nil {
				hrs.ByHandlingSite = getHttpRequestStats(byHandlingSite)
			}
			out[k] = hrs
		}
	}
	log.Printf("getHttpRequestsStats() => %#v\n", out)
	return out
}

func getTargetName(connectorName string) string {
	parts := strings.Split(connectorName, "@")
	if len(parts) > 0 {
		return parts[0]
	} else {
		return connectorName
	}
}

func getHttpProtocol(protocolVersion string) string {
	if protocolVersion == qdr.HttpVersion2 {
		return "http2"
	} else {
		return "http"
	}
}

func getTargetHost(iplookup *IpLookup, host string) string {
	result := iplookup.getPodName(host)
	if result == "" {
		return host
	} else {
		return result
	}
}

func getServiceStats(bridges []qdr.BridgeConfig, sites []Site, tcpconnections [][]qdr.TcpConnection, httpRequests [][]qdr.HttpRequestInfo, iplookup *IpLookup) []interface{} {
	tcpServices := TcpServiceStatsMap{}
	httpServices := HttpServiceStatsMap{}
	for _, b := range bridges {
		for _, c := range b.TcpConnectors {
			target := []ServiceTarget{
				ServiceTarget{
					Name:   getTargetHost(iplookup, c.Host),
					Target: getTargetName(c.Name),
					SiteId: c.SiteId,
				},
			}
			service, ok := tcpServices[c.Address]
			if ok {
				service.Targets = append(service.Targets, target...)
				tcpServices[c.Address] = service
			} else {
				service = TcpServiceStats{
					ServiceStats: ServiceStats{
						Address:  c.Address,
						Protocol: "tcp",
						Targets:  target,
					},
				}
				tcpServices[c.Address] = service
			}
		}
		for _, l := range b.TcpListeners {
			if _, ok := tcpServices[l.Address]; !ok {
				tcpServices[l.Address] = TcpServiceStats{
					ServiceStats: ServiceStats{
						Address:  l.Address,
						Protocol: "tcp",
					},
				}
			}

		}
		for _, c := range b.HttpConnectors {
			target := []ServiceTarget{
				ServiceTarget{
					Name:   getTargetHost(iplookup, c.Host),
					Target: getTargetName(c.Name),
					SiteId: c.SiteId,
				},
			}
			service, ok := httpServices[c.Address]
			if ok {
				service.Targets = append(service.Targets, target...)
				httpServices[c.Address] = service
			} else {
				service = HttpServiceStats{
					ServiceStats: ServiceStats{
						Address:  c.Address,
						Protocol: getHttpProtocol(c.ProtocolVersion),
						Targets:  target,
					},
					RequestsReceived: HttpRequestsReceivedList{},
					RequestsHandled:  HttpRequestsHandledList{},
				}
				httpServices[c.Address] = service
			}
		}
		for _, l := range b.HttpListeners {
			if _, ok := httpServices[l.Address]; !ok {
				httpServices[l.Address] = HttpServiceStats{
					ServiceStats: ServiceStats{
						Address:  l.Address,
						Protocol: getHttpProtocol(l.ProtocolVersion),
					},
					RequestsReceived: HttpRequestsReceivedList{},
					RequestsHandled:  HttpRequestsHandledList{},
				}
			}

		}
	}
	for i, c := range tcpconnections {
		tcpServices.updateTcpConnectionStats(sites[i].SiteId, c, iplookup)
	}
	for i, r := range httpRequests {
		httpServices.updateHttpRequestStats(sites[i].SiteId, r, iplookup)
	}

	services := []interface{}{}
	for _, s := range httpServices {
		services = append(services, s)
	}
	for _, s := range tcpServices {
		services = append(services, s)
	}
	return services
}

func getPeerIdentifier(addr string, iplookup *IpLookup) string {
	parts := strings.Split(addr, ":")
	peer := iplookup.getPodName(parts[0])
	if peer == "" {
		peer = parts[0]
	}
	return peer
}

type ConnectionStatsIndex map[string]map[string]ConnectionStats

func asConnectionStats(connection *qdr.TcpConnection, iplookup *IpLookup) ConnectionStats {
	stats := ConnectionStats{
		Id:        connection.Name,
		StartTime: connection.Uptime,
		LastOut:   connection.LastOut,
		LastIn:    connection.LastIn,
		BytesIn:   connection.BytesIn,
		BytesOut:  connection.BytesOut,
	}
	peer := getPeerIdentifier(connection.Host, iplookup)
	if connection.Direction == "in" {
		stats.Client = peer
	} else {
		stats.Server = peer
	}
	return stats
}

func (index ConnectionStatsIndex) updateTcpConnectionStats(c qdr.TcpConnection, iplookup *IpLookup) {
	byId, ok := index[c.Address]
	if ok {
		byId[c.Name] = asConnectionStats(&c, iplookup)
	} else {
		index[c.Address] = map[string]ConnectionStats{
			c.Name: asConnectionStats(&c, iplookup),
		}
	}
}

func (services TcpServiceStatsMap) updateTcpConnectionStats(siteId string, connections []qdr.TcpConnection, iplookup *IpLookup) {
	log.Printf("Updating tcp connection stats for %s %d", siteId, len(connections))
	ingress := ConnectionStatsIndex{}
	egress := ConnectionStatsIndex{}
	for _, c := range connections {
		if c.Direction == "in" {
			ingress.updateTcpConnectionStats(c, iplookup)
		} else {
			egress.updateTcpConnectionStats(c, iplookup)
		}
	}
	for _, service := range services {
		if ingress[service.Address] != nil {
			service.ConnectionsIngress = append(service.ConnectionsIngress, SiteConnections{
				SiteId:      siteId,
				Connections: ingress[service.Address],
			})
		}
		if egress[service.Address] != nil {
			service.ConnectionsEgress = append(service.ConnectionsEgress, SiteConnections{
				SiteId:      siteId,
				Connections: egress[service.Address],
			})
		}
		services[service.Address] = service
		log.Printf("Adding site connection stats for %s to %s (%d %d)", siteId, service.Address, len(service.ConnectionsIngress), len(service.ConnectionsEgress))
	}
}

type RequestStatsIndex map[string]map[string]HttpRequestStats

func asHttpRequestStats(r *qdr.HttpRequestInfo) HttpRequestStats {
	stats := HttpRequestStats{
		Requests:   r.Requests,
		LatencyMax: r.MaxLatency,
		BytesIn:    r.BytesIn,
		BytesOut:   r.BytesOut,
		Details:    r.Details,
	}
	if r.Direction == "in" {
		stats.ByHandlingSite = map[string]HttpRequestStats{
			r.Site: stats,
		}
	}
	return stats
}

func (index RequestStatsIndex) indexByHost(r *qdr.HttpRequestInfo, iplookup *IpLookup) {
	host := iplookup.getPodName(r.Host)
	if host == "" {
		host = r.Host
	}
	_, ok := index[r.Address]
	if ok {
		stats, ok := index[r.Address][host]
		if ok {
			s := asHttpRequestStats(r)
			stats.merge(&s)
		} else {
			stats = asHttpRequestStats(r)
		}
		index[r.Address][host] = stats
	} else {
		index[r.Address] = map[string]HttpRequestStats{
			host: asHttpRequestStats(r),
		}
	}
}

func (index RequestStatsIndex) indexBySite(r *qdr.HttpRequestInfo) {
	_, ok := index[r.Address]
	if ok {
		stats, ok := index[r.Address][r.Site]
		if ok {
			s := asHttpRequestStats(r)
			stats.merge(&s)
		} else {
			stats = asHttpRequestStats(r)
		}
		index[r.Address][r.Site] = stats
	} else {
		index[r.Address] = map[string]HttpRequestStats{
			r.Site: asHttpRequestStats(r),
		}
	}
}

func (services HttpServiceStatsMap) updateHttpRequestStats(siteId string, requests []qdr.HttpRequestInfo, iplookup *IpLookup) {
	log.Printf("Updating http request stats for %s %d", siteId, len(requests))
	byClient := RequestStatsIndex{}
	byServer := RequestStatsIndex{}
	byOriginatingSite := RequestStatsIndex{}
	for _, r := range requests {
		if r.Direction == "in" {
			byClient.indexByHost(&r, iplookup)
		} else {
			byServer.indexByHost(&r, iplookup)
			byOriginatingSite.indexBySite(&r)
		}
	}
	log.Printf("Indexed http request stats for %s by client %#v by server %#v", siteId, byClient, byServer)
	for _, service := range services {
		if byClient[service.Address] != nil {
			service.RequestsReceived = append(service.RequestsReceived, HttpRequestsReceived{
				SiteId:   siteId,
				ByClient: byClient[service.Address],
			})
		}
		if byServer[service.Address] != nil || byOriginatingSite[service.Address] != nil {
			service.RequestsHandled = append(service.RequestsHandled, HttpRequestsHandled{
				SiteId:            siteId,
				ByServer:          byServer[service.Address],
				ByOriginatingSite: byOriginatingSite[service.Address],
			})
		}
		services[service.Address] = service
		log.Printf("Adding site request stats for %s to %s (%d %d)", siteId, service.Address, len(service.RequestsReceived), len(service.RequestsHandled))
	}
}

type Site struct {
	SiteName  string   `json:"site_name"`
	SiteId    string   `json:"site_id"`
	Connected []string `json:"connected"`
	Namespace string   `json:"namespace"`
	Url       string   `json:"url"`
	Edge      bool     `json:"edge"`
}

func replace(in []string, lookup map[string]string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = lookup[s]
	}
	return out
}

func getSiteInfo(routers []qdr.Router) []Site {
	sites := map[string]Site{}
	lookup := map[string]string{}
	for _, r := range routers {
		if strings.Contains(r.Id, "skupper-router") {
			lookup[r.Id] = r.SiteId
		}
	}
	for _, r := range routers {
		if strings.Contains(r.Id, "skupper-router") {
			if site, ok := sites[r.SiteId]; ok {
				site.Connected = append(site.Connected, replace(r.ConnectedTo, lookup)...)
				sites[r.SiteId] = site
				log.Printf("Updating site %s based on router %s ", r.SiteId, r.Id)
			} else {
				sites[r.SiteId] = Site{
					SiteId:    r.SiteId,
					Connected: replace(r.ConnectedTo, lookup),
					Edge:      r.Edge,
				}
				log.Printf("Adding site %s based on router %s ", r.SiteId, r.Id)
			}
		} else {
			log.Printf("Skipping router %s", r.Id)
		}
	}
	sitelist := []Site{}
	for _, s := range sites {
		sitelist = append(sitelist, s)
	}
	log.Printf("Sites: %v %v", sites, sitelist)
	return sitelist
}

type ConsoleData struct {
	Sites    []Site        `json:"sites"`
	Services []interface{} `json:"services"`
}

func getSiteRouters(routers []qdr.Router) []qdr.Router {
	sites := map[string]qdr.Router{}
	for _, r := range routers {
		if strings.Contains(r.Id, "skupper-router") {
			sites[r.SiteId] = r
		}
	}
	list := []qdr.Router{}
	for _, r := range sites {
		list = append(list, r)
	}
	return list
}

func getConsoleData(agent *qdr.Agent, iplookup *IpLookup) (*ConsoleData, error) {
	routers, err := agent.GetAllRouters()
	if err != nil {
		return nil, fmt.Errorf("Error retrieving routers: %s", err)
	}
	//TODO: handle multiple routers per site, for now ensure we only have one router per site
	routers = getSiteRouters(routers)
	bridges, err := agent.GetBridges(routers)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving bridge configuration: %s", err)
	}
	tcpConns, err := agent.GetTcpConnections(routers)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving tcp connection stats: %s", err)
	}
	httpReqs, err := agent.GetHttpRequestInfo(routers)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving http request stats: %s", err)
	}
	log.Printf("Bridge data: %#v", bridges)
	data := ConsoleData{
		Sites: getSiteInfo(routers),
	}
	data.Services = getServiceStats(bridges, data.Sites, tcpConns, httpReqs, iplookup)
	//query each site for remaining information
	err = getAllSiteInfo(agent, data.Sites)
	if err != nil {
		return nil, fmt.Errorf("Error with site queries: %s", err)
	}
	return &data, nil
}
