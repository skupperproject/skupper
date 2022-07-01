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
	"text/tabwriter"
	"time"

	"github.com/gorilla/mux"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/qdr"
)

const (
	HttpInternalServerError string = "HttpServerError"
	HttpAuthFailure         string = "HttpAuthenticationFailure"
	SiteVersionConflict     string = "SiteVersionConflict"
)

type ConsoleServer struct {
	agentPool *qdr.AgentPool
	tokens    *TokenManager
	links     *LinkManager
	services  *ServiceManager
	policies  *PolicyManager
}

func newConsoleServer(cli *client.VanClient, config *tls.Config) *ConsoleServer {
	pool := qdr.NewAgentPool("amqps://"+types.QualifiedServiceName(types.LocalTransportServiceName, cli.Namespace)+":5671", config)
	return &ConsoleServer{
		agentPool: pool,
		tokens:    newTokenManager(cli),
		links:     newLinkManager(cli, pool),
		services:  newServiceManager(cli),
		policies:  newPolicyManager(cli),
	}
}

func authenticate(dir string, user string, password string) bool {
	filename := path.Join(dir, user)
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			event.Recordf(HttpAuthFailure, "Failed to authenticate %s, no such user exists", user)
		} else {
			event.Recordf(HttpAuthFailure, "Failed to authenticate %s: %s", user, err)
		}
		return false
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		event.Recordf(HttpAuthFailure, "Failed to authenticate %s: %s", user, err)
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

type VersionInfo struct {
	ServiceControllerVersion string `json:"service_controller_version"`
	RouterVersion            string `json:"router_version"`
	SiteVersion              string `json:"site_version"`
}

func (server *ConsoleServer) httpInternalError(w http.ResponseWriter, err error) {
	event.Record(HttpInternalServerError, err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (server *ConsoleServer) version() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := VersionInfo{
			ServiceControllerVersion: client.Version,
		}
		agent, err := server.agentPool.Get()
		if err != nil {
			server.httpInternalError(w, fmt.Errorf("Could not get management agent : %s", err))
			return
		}
		router, err := agent.GetLocalRouter()
		server.agentPool.Put(agent)
		if err != nil {
			server.httpInternalError(w, fmt.Errorf("Error retrieving local router version: %s", err))
			return
		}
		v.RouterVersion = router.Version
		v.SiteVersion = router.Site.Version
		if wantsJsonOutput(r) {
			bytes, err := json.MarshalIndent(v, "", "    ")
			if err != nil {
				server.httpInternalError(w, fmt.Errorf("Error writing version: %s", err))
				return
			}
			fmt.Fprintf(w, string(bytes)+"\n")
		} else {
			tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
			fmt.Fprintln(tw, "site\t"+v.SiteVersion)
			fmt.Fprintln(tw, "service-controller\t"+v.ServiceControllerVersion)
			fmt.Fprintln(tw, "router\t"+v.RouterVersion)
			tw.Flush()
		}
	})
}

func (server *ConsoleServer) site() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, os.Getenv("SKUPPER_SITE_ID"))
	})
}

const (
	MaxFieldLength int = 60
)

func wrap(text string, width int) []string {
	words := strings.Fields(text)
	wrapped := []string{}
	line := ""
	for _, word := range words {
		if len(word)+len(line)+1 > width {
			wrapped = append(wrapped, line)
			line = word
		} else {
			if line == "" {
				line = word
			} else {
				line = line + " " + word
			}
		}
	}
	wrapped = append(wrapped, line)
	return wrapped
}

func wantsJsonOutput(r *http.Request) bool {
	options := r.URL.Query()
	output := options.Get("output")
	return output == "json"
}

func (server *ConsoleServer) serveEvents() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e := event.Query()
		if wantsJsonOutput(r) {
			bytes, err := json.MarshalIndent(e, "", "    ")
			if err != nil {
				server.httpInternalError(w, fmt.Errorf("Error writing events: %s", err))
				return
			}
			fmt.Fprintf(w, string(bytes)+"\n")
		} else {
			tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s", "NAME", "COUNT", " ", "AGE"))
			for _, group := range e {
				fmt.Fprintln(tw, fmt.Sprintf("%s\t%d\t%s\t%s", group.Name, group.Total, " ", time.Since(group.LastOccurrence).Round(time.Second)))
				for _, detail := range group.Counts {
					if len(detail.Key) > MaxFieldLength {
						lines := wrap(detail.Key, MaxFieldLength)
						for i, line := range lines {
							if i == 0 {
								fmt.Fprintln(tw, fmt.Sprintf("%s\t%d\t%s\t%s", " ", detail.Count, line, time.Since(detail.LastOccurrence).Round(time.Second)))
							} else {
								fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s", " ", " ", line, ""))
							}
						}
					} else {
						fmt.Fprintln(tw, fmt.Sprintf("%s\t%d\t%s\t%s", " ", detail.Count, detail.Key, time.Since(detail.LastOccurrence).Round(time.Second)))
					}
				}
			}
			tw.Flush()
		}
	})
}

func (server *ConsoleServer) serveSites() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := server.getData(w)
		if d != nil {
			if wantsJsonOutput(r) {
				bytes, err := json.MarshalIndent(d.Sites, "", "    ")
				if err != nil {
					server.httpInternalError(w, fmt.Errorf("Error writing json: %s", err))
				} else {
					fmt.Fprintf(w, string(bytes)+"\n")
				}
			} else {
				tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
				fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", "ID", "NAME", "EDGE", "VERSION", "NAMESPACE", "URL", "CONNECTED TO"))
				for _, site := range d.Sites {
					siteVersion := site.Version
					if err := server.links.cli.VerifySiteCompatibility(site.Version); err != nil {
						siteVersion += fmt.Sprintf(" (incompatible - %v)", err)
					}
					fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%t\t%s\t%s\t%s\t%s", site.SiteId, site.SiteName, site.Edge, siteVersion, site.Namespace, site.Url, strings.Join(site.Connected, " ")))
				}
				tw.Flush()

			}
		}
	})
}

func (server *ConsoleServer) serveServices() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := server.getData(w)
		if d != nil {
			if wantsJsonOutput(r) {
				bytes, err := json.MarshalIndent(d.Services, "", "    ")
				if err != nil {
					server.httpInternalError(w, fmt.Errorf("Error writing json: %s", err))
				} else {
					fmt.Fprintf(w, string(bytes)+"\n")
				}
			} else {
				tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
				fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s", "ADDRESS", "PROTOCOL", "TARGET", "SITE"))
				for _, s := range d.Services {
					var service *data.Service
					if hs, ok := s.(data.HttpService); ok {
						service = &hs.Service
					}
					if ts, ok := s.(data.TcpService); ok {
						service = &ts.Service
					}
					if service != nil {
						fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s", service.Address, service.Protocol, "", ""))
						for _, target := range service.Targets {
							fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s", "", "", target.Name, target.SiteId))
						}
					}
				}
				tw.Flush()
			}
		}
	})
}

func removeEmpty(input []string) []string {
	output := []string{}
	for _, s := range input {
		if s != "" {
			output = append(output, s)
		}
	}
	return output
}

func (server *ConsoleServer) checkService() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, err := server.agentPool.Get()
		if err != nil {
			server.httpInternalError(w, fmt.Errorf("Could not get management agent : %s", err))
		} else {
			// what is the name of the service to check?
			vars := mux.Vars(r)
			if address, ok := vars["name"]; ok {
				data, err := checkService(agent, address)
				server.agentPool.Put(agent)
				if err != nil {
					server.httpInternalError(w, err)
				} else {
					if wantsJsonOutput(r) {
						bytes, err := json.MarshalIndent(data, "", "    ")
						if err != nil {
							server.httpInternalError(w, fmt.Errorf("Error writing json: %s", err))
						} else {
							fmt.Fprintf(w, string(bytes)+"\n")
						}
					} else {
						if len(data.Observations) > 0 {
							for _, observation := range data.Observations {
								fmt.Fprintln(w, observation)
							}
							if data.HasDetailObservations() {
								fmt.Fprintln(w, "")
								fmt.Fprintln(w, "Details:")
								fmt.Fprintln(w, "")
								tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
								for _, site := range data.Details {
									for i, observation := range site.Observations {
										if i == 0 {
											fmt.Fprintln(tw, fmt.Sprintf("%s\t%s", site.SiteId, observation))
										} else {
											fmt.Fprintln(tw, fmt.Sprintf("%s\t%s", "", observation))
										}
									}
								}
								tw.Flush()
							}
						} else {
							fmt.Fprintln(w, "No issues found")
						}
					}
				}
			} else {
				http.Error(w, "Invalid path", http.StatusNotFound)
			}
		}
	})
}

func (server *ConsoleServer) getData(w http.ResponseWriter) *data.ConsoleData {
	agent, err := server.agentPool.Get()
	if err != nil {
		server.httpInternalError(w, fmt.Errorf("Could not get management agent : %s", err))
		return nil
	}
	data, err := getConsoleData(agent)
	server.agentPool.Put(agent)
	if err != nil {
		server.httpInternalError(w, err)
		return nil
	}
	return data
}

func (server *ConsoleServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := server.getData(w)
	if data != nil {
		bytes, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			server.httpInternalError(w, fmt.Errorf("Error writing json: %s", err))
		} else {
			fmt.Fprintf(w, string(bytes)+"\n")
		}
	}
}

func (server *ConsoleServer) writeJson(obj interface{}, w http.ResponseWriter) {
	bytes, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		server.httpInternalError(w, fmt.Errorf("Error writing json: %s", err))
	} else {
		fmt.Fprintf(w, string(bytes)+"\n")
	}
}

func (server *ConsoleServer) start(stopCh <-chan struct{}) error {
	go server.listen()
	go server.listenLocal()
	return nil
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE")
		next.ServeHTTP(w, r)
	})
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
	r := mux.NewRouter()
	r.Handle("/DATA", authenticated(server))
	r.Handle("/tokens", authenticated(serveTokens(server.tokens)))
	r.Handle("/tokens/", authenticated(serveTokens(server.tokens)))
	r.Handle("/tokens/{name}", authenticated(serveTokens(server.tokens)))
	r.Handle("/downloadclaim/{name}", authenticated(downloadClaim(server.tokens)))
	r.Handle("/links", authenticated(serveLinks(server.links)))
	r.Handle("/links/", authenticated(serveLinks(server.links)))
	r.Handle("/links/{name}", authenticated(serveLinks(server.links)))
	r.Handle("/services", authenticated(serveServices(server.services)))
	r.Handle("/services/", authenticated(serveServices(server.services)))
	r.Handle("/services/{name}", authenticated(serveServices(server.services)))
	r.Handle("/targets", authenticated(serveTargets(server.services)))
	r.Handle("/targets/", authenticated(serveTargets(server.services)))
	r.Handle("/version", authenticated(server.version()))
	r.Handle("/site", authenticated(server.site()))
	r.Handle("/events", authenticated(server.serveEvents()))
	r.Handle("/policy/expose/{resourceType}/{resourceName}", authenticated(server.policies.expose()))
	r.Handle("/policy/service/{name}", authenticated(server.policies.service()))
	r.Handle("/policy/incominglink", authenticated(server.policies.incomingLink()))
	r.Handle("/policy/outgoinglink/{hostname}", authenticated(server.policies.outgoingLink()))
	r.Handle("/servicecheck/{name}", authenticated(server.checkService()))
	r.PathPrefix("/").Handler(authenticated(http.FileServer(http.Dir("/app/console/"))))
	if os.Getenv("USE_CORS") != "" {
		r.Use(cors)
	}
	_, err := os.Stat("/etc/service-controller/console/tls.crt")
	if err == nil {
		log.Fatal(http.ListenAndServeTLS(addr, "/etc/service-controller/console/tls.crt", "/etc/service-controller/console/tls.key", r))
	} else {
		log.Fatal(http.ListenAndServe(addr, r))
	}
}

func (server *ConsoleServer) listenLocal() {
	addr := "localhost:8181"
	r := mux.NewRouter()
	r.Handle("/DATA", server)
	r.Handle("/version", server.version())
	r.Handle("/events", server.serveEvents())
	r.Handle("/sites", server.serveSites())
	r.Handle("/services", server.serveServices())
	r.Handle("/servicecheck/{name}", server.checkService())
	r.Handle("/policy/expose/{resourceType}/{resourceName}", server.policies.expose())
	r.Handle("/policy/service/{name}", server.policies.service())
	r.Handle("/policy/incominglink", server.policies.incomingLink())
	r.Handle("/policy/outgoinglink/{hostname}", server.policies.outgoingLink())
	log.Fatal(http.ListenAndServe(addr, r))
}

func set(m map[string]map[string]bool, k1 string, k2 string) {
	m2, ok := m[k1]
	if !ok {
		m2 = map[string]bool{}
	}
	m2[k2] = true
	m[k1] = m2
}

func getAllSites(routers []qdr.Router) []data.SiteQueryData {
	sites := map[string]data.SiteQueryData{}
	routerToSite := map[string]string{}
	siteConnections := map[string]map[string]bool{}
	for _, r := range routers {
		routerToSite[r.Id] = r.Site.Id
		site, exists := sites[r.Site.Id]
		if !exists {
			if !r.IsGateway() {
				sites[r.Site.Id] = data.SiteQueryData{
					Site: data.Site{
						SiteId:    r.Site.Id,
						Version:   r.Site.Version,
						Edge:      r.Edge && strings.Contains(r.Id, "skupper-router"),
						Connected: []string{},
						Gateway:   false,
					},
				}
			}
		} else if r.Site.Version != site.Version {
			event.Recordf(SiteVersionConflict, "Conflicting site version for %s: %s != %s", site.SiteId, site.Version, r.Site.Version)
		}
	}
	for _, r := range routers {
		for _, id := range r.ConnectedTo {
			set(siteConnections, r.Site.Id, routerToSite[id])
		}
	}
	list := []data.SiteQueryData{}
	for _, s := range sites {
		m := siteConnections[s.SiteId]
		for key, _ := range m {
			if key != s.SiteId {
				s.Connected = append(s.Connected, key)
			}
		}
		list = append(list, s)
	}
	return list
}

func getConsoleData(agent *qdr.Agent) (*data.ConsoleData, error) {
	routers, err := agent.GetAllRouters()
	if err != nil {
		return nil, fmt.Errorf("Error retrieving routers: %s", err)
	}
	sites := getAllSites(routers)
	querySites(agent, sites)
	for i, s := range sites {
		if s.Version == "" {
			// prior to 0.5 there was no version in router metadata
			// and site query did not return services, so they are
			// retrieved here separately
			err = getServiceInfo(agent, routers, &sites[i], data.NewNullNameMapping())
			if err != nil {
				return nil, fmt.Errorf("Error retrieving service data from old site %s: %s", s.SiteId, err)
			}
		}
	}
	gateways := queryGateways(agent, sites)
	sites = append(sites, gateways...)
	consoleData := &data.ConsoleData{}
	consoleData.Merge(sites)
	return consoleData, nil
}

func checkService(agent *qdr.Agent, address string) (*data.ServiceCheck, error) {
	// get all routers of version 0.5 and up
	routers, err := agent.GetAllRouters()
	if err != nil {
		return nil, fmt.Errorf("Error retrieving routers: %s", err)
	}
	allSites := getAllSites(routers)
	serviceCheck := data.ServiceCheck{}
	sites := map[string]data.Site{}
	for _, site := range allSites {
		if site.Version != "" {
			sites[site.SiteId] = site.Site
			serviceCheck.Details = append(serviceCheck.Details, data.ServiceDetail{
				SiteId: site.SiteId,
			})
		}
	}
	err = checkServiceForSites(agent, address, &serviceCheck)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving service detail: %s", err)
	}
	return &serviceCheck, nil
}
