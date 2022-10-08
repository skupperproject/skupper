package podman

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type RouterEntityManagerPodman struct {
	cli *podman.PodmanRestClient
}

func NewRouterEntityManagerPodman(cli *podman.PodmanRestClient) *RouterEntityManagerPodman {
	return &RouterEntityManagerPodman{
		cli: cli,
	}
}

func (r *RouterEntityManagerPodman) exec(cmd []string) (string, error) {
	return r.cli.ContainerExec(types.TransportDeploymentName, cmd)
}

func (r *RouterEntityManagerPodman) CreateSslProfile(sslProfile qdr.SslProfile) error {
	cmd := qdr.SkmanageCreateCommand("sslProfile", sslProfile.Name, sslProfile)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error creating sslProfile %s - %w", sslProfile.Name, err)
	}
	return nil
}

func (r *RouterEntityManagerPodman) DeleteSslProfile(name string) error {
	cmd := qdr.SkmanageDeleteCommand("sslProfile", name)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error deleting sslProfile %s - %w", name, err)
	}
	return nil
}

func (r *RouterEntityManagerPodman) CreateConnector(connector qdr.Connector) error {
	cmd := qdr.SkmanageCreateCommand("connector", connector.Name, connector)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error creating connector %s - %w", connector.Name, err)
	}
	return nil
}

func (r *RouterEntityManagerPodman) DeleteConnector(name string) error {
	cmd := qdr.SkmanageDeleteCommand("connector", name)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error deleting sslProfile %s - %w", name, err)
	}
	return nil
}

func (r *RouterEntityManagerPodman) QueryConnections(routerId string, edge bool) ([]qdr.Connection, error) {
	cmd := qdr.SkmanageQueryCommand("connection", routerId, edge, "")
	var data string
	var err error
	if data, err = r.exec(cmd); err != nil {
		return nil, fmt.Errorf("error querying connections - %w", err)
	}
	var connections []qdr.Connection
	ioutil.WriteFile("/tmp/baddata", []byte(data), 0644)
	err = json.Unmarshal([]byte(data), &connections)
	if err != nil {
		return nil, fmt.Errorf("error retrieving connections - %w", err)
	}
	return connections, nil
}

func (r *RouterEntityManagerPodman) QueryAllRouters() ([]qdr.Router, error) {
	var routersToQuery []qdr.Router
	var routersRet []qdr.Router
	routerNodes, err := r.QueryRouterNodes()
	if err != nil {
		return nil, err
	}
	edgeRouters, err := r.QueryEdgeRouters()
	if err != nil {
		return nil, err
	}
	for _, r := range routerNodes {
		routersToQuery = append(routersToQuery, *r.AsRouter())
	}
	for _, r := range edgeRouters {
		routersToQuery = append(routersToQuery, r)
	}
	for _, router := range routersToQuery {
		// querying io.skupper.router.router to retrieve version for all routers found
		routerToQuery := router.Id
		cmd := qdr.SkmanageQueryCommand("io.skupper.router.router", routerToQuery, router.Edge, "")
		rJson, err := r.cli.ContainerExec(types.TransportDeploymentName, cmd)
		if err != nil {
			return nil, fmt.Errorf("error querying router info from %s - %w", routerToQuery, err)
		}
		var records []qdr.Record
		err = json.Unmarshal([]byte(rJson), &records)
		if err != nil {
			return nil, fmt.Errorf("error decoding router info from %s - %w", routerToQuery, err)
		}
		router.Site = qdr.GetSiteMetadata(records[0].AsString("metadata"))

		// retrieving connections
		conns, err := r.QueryConnections(routerToQuery, router.Edge)
		if err != nil {
			return nil, fmt.Errorf("error querying router connections from %s - %w", routerToQuery, err)
		}
		for _, conn := range conns {
			if conn.Role == types.InterRouterRole && conn.Dir == qdr.DirectionOut {
				router.ConnectedTo = append(router.ConnectedTo, conn.Container)
			} else if conn.Role == types.EdgeRole && conn.Dir == qdr.DirectionIn {
				router.ConnectedTo = append(router.ConnectedTo, conn.Container)
			}
		}
		routersRet = append(routersRet, router)
	}
	return routersRet, nil
}

func (r *RouterEntityManagerPodman) QueryRouterNodes() ([]qdr.RouterNode, error) {
	var routerNodes []qdr.RouterNode
	// Retrieving all connections
	conns, err := r.QueryConnections("", false)
	if err != nil {
		return nil, fmt.Errorf("error retrieving router connections - %w", err)
	}
	// Retrieving Router nodes
	var routerId string
	var edge bool
	for _, conn := range conns {
		if conn.Role == types.ConnectorRoleEdge && conn.Dir == qdr.DirectionOut {
			routerId = conn.Container
			edge = true
			break
		}
	}
	cmd := qdr.SkmanageQueryCommand("io.skupper.router.router.node", routerId, edge, "")
	routerNodesJson, err := r.cli.ContainerExec(types.TransportDeploymentName, cmd)
	if err != nil {
		return nil, fmt.Errorf("error querying router nodes - %w", err)
	}
	err = json.Unmarshal([]byte(routerNodesJson), &routerNodes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse router nodes - %w", err)
	}
	return routerNodes, nil
}

func (r *RouterEntityManagerPodman) QueryEdgeRouters() ([]qdr.Router, error) {
	var routers []qdr.Router
	routerNodes, err := r.QueryRouterNodes()
	if err != nil {
		return nil, fmt.Errorf("error querying router nodes - %w", err)
	}
	for _, routerNode := range routerNodes {
		conns, err := r.QueryConnections(routerNode.Id, false)
		if err != nil {
			return nil, fmt.Errorf("error querying connections from router %s - %w", routerNode.Id, err)
		}
		for _, conn := range conns {
			if conn.Role == types.EdgeRole && conn.Dir == qdr.DirectionIn {
				routers = append(routers, qdr.Router{
					Id:          conn.Container,
					Address:     qdr.GetRouterAddress(conn.Container, true),
					Edge:        true,
					ConnectedTo: []string{routerNode.Id},
				})
			}
		}
	}
	return routers, nil
}
