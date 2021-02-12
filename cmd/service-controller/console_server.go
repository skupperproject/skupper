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
}

func newConsoleServer(cli *client.VanClient, config *tls.Config) *ConsoleServer {
	return &ConsoleServer{
		agentPool: qdr.NewAgentPool("amqps://skupper-messaging:5671", config),
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
		bytes, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			server.httpInternalError(w, fmt.Errorf("Error writing version: %s", err))
			return
		}
		fmt.Fprintf(w, string(bytes)+"\n")
	})
}

func (server *ConsoleServer) serveEvents() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e := event.Query()
		bytes, err := json.MarshalIndent(e, "", "    ")
		if err != nil {
			server.httpInternalError(w, fmt.Errorf("Error writing events: %s", err))
			return
		}
		fmt.Fprintf(w, string(bytes)+"\n")
	})
}

func (server *ConsoleServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	agent, err := server.agentPool.Get()
	if err != nil {
		server.httpInternalError(w, fmt.Errorf("Could not get management agent : %s", err))
		return
	}
	data, err := getConsoleData(agent)
	server.agentPool.Put(agent)
	if err != nil {
		server.httpInternalError(w, fmt.Errorf("Error retrieving console data: %s", err))
		return
	}
	bytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		server.httpInternalError(w, fmt.Errorf("Error writing json: %s", err))
		return
	}
	fmt.Fprintf(w, string(bytes)+"\n")
}

func (server *ConsoleServer) start(stopCh <-chan struct{}) error {
	go server.listen()
	return nil
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
	http.Handle("/Version", authenticated(server.version()))
	http.Handle("/Events", authenticated(server.serveEvents()))
	http.Handle("/", authenticated(http.FileServer(http.Dir("/app/console/"))))
	log.Fatal(http.ListenAndServe(addr, nil))
}

func (server *ConsoleServer) listenLocal() {
	addr := "localhost:8181"
	mux := http.NewServeMux()
	mux.Handle("/DATA", server)
	mux.Handle("/Version", server.version())
	http.Handle("/Events", server.serveEvents())
	log.Fatal(http.ListenAndServe(addr, mux))
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
			sites[r.Site.Id] = data.SiteQueryData{
				Site: data.Site{
					SiteId:    r.Site.Id,
					Version:   r.Site.Version,
					Edge:      r.Edge && strings.Contains(r.Id, "skupper-router"),
					Connected: []string{},
				},
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
			s.Connected = append(s.Connected, key)
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
	consoleData := &data.ConsoleData{}
	consoleData.Merge(sites)
	return consoleData, nil
}
