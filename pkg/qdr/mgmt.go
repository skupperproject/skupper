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

	connections, err := GetConnections(namespace, clientset, config)
	if err != nil {
		return result, err
	}
	// Go through this list once to add all of its
	// members to the directly-connected list...
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == "out" {
			nodeName := fmt.Sprintf("router.node/%s", c.Container)
			direct[nodeName] = true
		}
	}
	// ...and go through it again to get all of
	// the indirect nodes.
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == "out" {
			uplinkNodes, err := getNodesForRouter(c.Container, namespace, clientset, config)
			if err != nil {
				return result, err
			}
			for _, uplinkNode := range uplinkNodes {
				// Don't count a node as being indirectly connected
				// if we already know that it is directly connected.
				if _, present := direct[uplinkNode.Name]; !present {
					indirect[uplinkNode.Name] = true
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
	for _, n := range nodes {
		edges, err := GetEdgeSitesForRouter(n.Id, namespace, clientset, config)
		if err != nil {
			return result, fmt.Errorf("Failed to check edge nodes for %s: %w", n.Id, err)
		}
		if n.NextHop == "(self)" {
			result.Direct += edges
		} else {
			result.Indirect += edges
			if n.NextHop == "" {
				result.Direct++
			} else {
				result.Indirect++
			}
		}
	}
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
			result, err = getConnectedSitesFromNodesInterior(nodes, namespace, clientset, config)
			if err != nil {
				return result, err
			}
			return result, nil
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
		return nil, err
	} else {
		return &buffer, nil
	}
}
