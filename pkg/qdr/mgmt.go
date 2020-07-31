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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

type RouterNode struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	NextHop string `json:"nextHop"`
	Address string `json:"address"`
}

type Connection struct {
	Container  string `json:"container"`
	OperStatus string `json:"operStatus"`
	Host       string `json:"host"`
	Role       string `json:"role"`
	Active     bool   `json:"active"`
	Dir        string `json:"dir"`
}

func getConnectedSitesFromNodesEdge(namespace string, clientset kubernetes.Interface, config *restclient.Config) (types.TransportConnectedSites, error) {
	result := types.TransportConnectedSites{}
	direct := make(map[string]bool)
	indirect := make(map[string]bool)
	interiors := make(map[string]RouterNode)

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
		} else {
			for _, edge := range edges {
				key := fmt.Sprintf("router.node/%s", edge.Container)
				if _, present := direct[key]; !present && edge.Container != localId {
					indirect[key] = true
				}
			}
		}
	}
	result.Direct = len(direct)
	result.Indirect = len(indirect)
	result.Total = result.Direct + result.Indirect
	return result, nil
}

func getConnectedSitesFromNodesInterior(nodes []RouterNode, namespace string, clientset kubernetes.Interface, config *restclient.Config) (types.TransportConnectedSites, error) {
	result := types.TransportConnectedSites{}
	direct := make(map[string]bool)
	indirect := make(map[string]bool)
	for _, n := range nodes {
		if n.NextHop == "(self)" {
			edges, err := getEdgeConnectionsForInterior(n.Id, namespace, clientset, config)
			if err != nil {
				return result, fmt.Errorf("Failed to check edge nodes for %s: %w", n.Id, err)
			}
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
		"qdmanage",
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

func GetNodes(namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]RouterNode, error) {
	return getNodesForRouter("", namespace, clientset, config)
}

func getNodesForRouter(routerid, namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]RouterNode, error) {
	command := get_query_for_router("node", routerid)
	buffer, err := router_exec(command, namespace, clientset, config)
	if err != nil {
		return nil, err
	} else {
		results := []RouterNode{}
		err = json.Unmarshal(buffer.Bytes(), &results)
		if err != nil {
			fmt.Println("Failed to parse JSON:", err, buffer.String())
			return nil, err
		} else {
			return results, nil
		}
	}
}

func GetInterRouterOrEdgeConnection(host string, connections []Connection) *Connection {
	for _, c := range connections {
		if (c.Role == "inter-router" || c.Role == "edge") && c.Host == host {
			return &c
		}
	}
	return nil
}

func GetConnections(namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]Connection, error) {
	return getConnectionsForRouter("", namespace, clientset, config)
}

func getEdgeUplinkConnections(namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]Connection, error) {
	connections, err := GetConnections(namespace, clientset, config)
	if err != nil {
		return nil, err
	}

	return getEdgeConnections("out", connections)
}

func getEdgeConnectionsForInterior(routerid string, namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]Connection, error) {
	connections, err := getConnectionsForRouter(routerid, namespace, clientset, config)
	if err != nil {
		return nil, err
	}

	return getEdgeConnections("in", connections)
}

func getEdgeConnections(direction string, connections []Connection) ([]Connection, error) {
	result := []Connection{}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == direction {
			result = append(result, c)
		}
	}
	return result, nil
}

func getConnectionsForRouter(routerid string, namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]Connection, error) {
	command := get_query_for_router("connection", routerid)
	buffer, err := router_exec(command, namespace, clientset, config)
	if err != nil {
		return nil, err
	} else {
		results := []Connection{}
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

	var stdout io.Writer

	buffer := bytes.Buffer{}
	stdout = bufio.NewWriter(&buffer)

	restClient, err := restclient.RESTClientFor(config)
	if err != nil {
		panic(err)
	}

	req := restClient.Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: pod.Spec.Containers[0].Name,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    false,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		panic(err)
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("Error executing %s: %v", command, err)
	} else {
		return &buffer, nil
	}
}
