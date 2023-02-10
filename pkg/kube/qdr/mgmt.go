/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package qdr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func getConnectedSitesFromNodesEdge(namespace string, clientset kubernetes.Interface, config *restclient.Config) (types.TransportConnectedSites, error) {
	result := types.TransportConnectedSites{}
	direct := make(map[string]bool)
	indirect := make(map[string]bool)
	interiors := make(map[string]qdr.RouterNode)

	uplinks, err := getEdgeUplinkConnections(namespace, clientset, config)
	if err != nil {
		return result, err
	}
	// Go through this list once to add all of its
	// members to the directly-connected list...
	for _, c := range uplinks {
		nodeName := fmt.Sprintf("router.node/%s", c.Container)
		direct[nodeName] = true
	}
	// ...and go through it again to get all of
	// the indirect nodes.
	interiorsRetrieved := false
	for _, c := range uplinks {
		if interiorsRetrieved {
			key := fmt.Sprintf("router.node/%s", c.Container)
			if _, ok := interiors[key]; !ok {
				result.Warnings = append(result.Warnings, "There are edge uplinks to distinct networks, please verify topology (connected counts may not be accurate).")
				continue
			}
		}
		interiorNodes, err := getNodesForRouter(c.Container, namespace, clientset, config)
		if err != nil {
			return result, err
		} else {
			interiorsRetrieved = true
			for _, interiorNode := range interiorNodes {
				if _, present := interiors[interiorNode.Name]; !present {
					interiors[interiorNode.Name] = interiorNode
				}
				// Don't count a node as being indirectly connected
				// if we already know that it is directly connected.
				if _, present := direct[interiorNode.Name]; !present {
					indirect[interiorNode.Name] = true
				}
			}
		}
	}
	localId, err := getLocalRouterId(namespace, clientset, config)
	if err != nil {
		return result, err
	}
	for _, interiorNode := range interiors {
		edges, err := getEdgeConnectionsForInterior(interiorNode.Id, namespace, clientset, config)
		if err != nil {
			return result, err
		}
		edges = filterSiteRouters(edges)
		for _, edge := range edges {
			key := fmt.Sprintf("router.node/%s", edge.Container)
			if _, present := direct[key]; !present && edge.Container != localId {
				indirect[key] = true
			}
		}
	}
	result.Direct = len(direct)
	result.Indirect = len(indirect)
	result.Total = result.Direct + result.Indirect
	return result, nil
}

func getConnectedSitesFromNodesInterior(nodes []qdr.RouterNode, namespace string, clientset kubernetes.Interface, config *restclient.Config) (types.TransportConnectedSites, error) {
	result := types.TransportConnectedSites{}
	direct := make(map[string]bool)
	indirect := make(map[string]bool)
	for _, n := range nodes {
		if n.NextHop == "(self)" {
			edges, err := getEdgeConnectionsForInterior(n.Id, namespace, clientset, config)
			if err != nil {
				return result, fmt.Errorf("Failed to check edge nodes for %s: %w", n.Id, err)
			}
			edges = filterSiteRouters(edges)
			for _, edge := range edges {
				if _, present := direct[edge.Container]; !present {
					direct[edge.Container] = true
				}
			}
			break
		}
	}
	for _, n := range nodes {
		if n.NextHop != "(self)" {
			edges, err := getEdgeConnectionsForInterior(n.Id, namespace, clientset, config)
			if err != nil {
				return result, fmt.Errorf("Failed to check edge nodes for %s: %w", n.Id, err)
			}
			edges = filterSiteRouters(edges)
			for _, edge := range edges {
				if _, present := direct[edge.Container]; !present {
					if _, present = indirect[edge.Container]; !present {
						indirect[edge.Container] = true
					}
				}
			}
			if n.NextHop == "" {
				direct[n.Id] = true
			} else {
				indirect[n.Id] = true
			}
		}
	}
	result.Direct = len(direct)
	result.Indirect = len(indirect)
	result.Total = result.Direct + result.Indirect
	return result, nil
}

func GetConnectedSites(edge bool, namespace string, clientset kubernetes.Interface, config *restclient.Config) (types.TransportConnectedSites, error) {
	result := types.TransportConnectedSites{}
	if edge {
		return getConnectedSitesFromNodesEdge(namespace, clientset, config)
	} else {
		nodes, err := GetNodes(namespace, clientset, config)
		if err == nil {
			return getConnectedSitesFromNodesInterior(nodes, namespace, clientset, config)
		} else {
			return result, err
		}
	}
}

func GetEdgeSitesForRouter(routerid string, namespace string, clientset kubernetes.Interface, config *restclient.Config) (int, error) {
	connections, err := getConnectionsForRouter(routerid, namespace, clientset, config)

	if err == nil {
		count := 0
		for _, c := range connections {
			if c.Role == "edge" && c.Dir == "in" {
				count++
			}
		}
		return count, nil
	} else {
		return 0, err
	}
}

func get_query(typename string) []string {
	return []string{
		"skmanage",
		"query",
		"--type",
		typename,
	}
}

func get_query_for_router(typename string, routerid string) []string {
	results := get_query(typename)
	if routerid != "" {
		results = append(results, "--router", routerid)
	}
	return results
}

func GetNodes(namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]qdr.RouterNode, error) {
	return getNodesForRouter("", namespace, clientset, config)
}

func getNodesForRouter(routerid, namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]qdr.RouterNode, error) {
	command := get_query_for_router("node", routerid)
	buffer, err := router_exec(command, namespace, clientset, config)
	if err != nil {
		return nil, err
	} else {
		results := []qdr.RouterNode{}
		err = json.Unmarshal(buffer.Bytes(), &results)
		if err != nil {
			fmt.Println("Failed to parse JSON:", err, buffer.String())
			return nil, err
		} else {
			return results, nil
		}
	}
}

func GetConnections(namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]qdr.Connection, error) {
	return getConnectionsForRouter("", namespace, clientset, config)
}

func filterSiteRouters(in []qdr.Connection) []qdr.Connection {
	results := []qdr.Connection{}
	for _, c := range in {
		if strings.Contains(c.Container, "skupper-router") {
			results = append(results, c)
		}
	}
	return results
}

func getEdgeUplinkConnections(namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]qdr.Connection, error) {
	connections, err := GetConnections(namespace, clientset, config)
	if err != nil {
		return nil, err
	}

	return getEdgeConnections("out", connections)
}

func getEdgeConnectionsForInterior(routerid string, namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]qdr.Connection, error) {
	connections, err := getConnectionsForRouter(routerid, namespace, clientset, config)
	if err != nil {
		return nil, err
	}

	return getEdgeConnections("in", connections)
}

func getEdgeConnections(direction string, connections []qdr.Connection) ([]qdr.Connection, error) {
	result := []qdr.Connection{}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == direction {
			result = append(result, c)
		}
	}
	return result, nil
}

func getConnectionsForRouter(routerid string, namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]qdr.Connection, error) {
	command := get_query_for_router("connection", routerid)
	buffer, err := router_exec(command, namespace, clientset, config)
	if err != nil {
		return nil, err
	} else {
		results := []qdr.Connection{}
		err = json.Unmarshal(buffer.Bytes(), &results)
		if err != nil {
			fmt.Println("Failed to parse JSON:", err, buffer.String())
			return nil, err
		} else {
			return results, nil
		}
	}
}

func getLocalRouterId(namespace string, clientset kubernetes.Interface, config *restclient.Config) (string, error) {
	command := get_query("router")
	buffer, err := router_exec(command, namespace, clientset, config)
	if err != nil {
		return "", err
	} else {
		results := []interface{}{}
		err = json.Unmarshal(buffer.Bytes(), &results)
		if err != nil {
			return "", fmt.Errorf("Failed to parse JSON: %s %q", err, buffer.String())
		} else {
			if router, ok := results[0].(map[string]interface{}); ok {
				if id, ok := router["id"].(string); ok {
					return id, nil
				}
			}
			return "", fmt.Errorf("Could not get router id from %#v", results)
		}
	}
}

func router_exec(command []string, namespace string, clientset kubernetes.Interface, config *restclient.Config) (*bytes.Buffer, error) {
	pod, err := kube.GetReadyPod(namespace, clientset, "router")
	if err != nil {
		return nil, err
	}
	return kube.ExecCommandInContainer(command, pod.Name, "router", namespace, clientset, config)
}
