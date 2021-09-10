package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gorilla/mux"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
)

const (
	ServiceManagement string = "ServiceManagement"
)

type PortDescription struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

func asPortDescriptions(in []corev1.ContainerPort) []PortDescription {
	out := []PortDescription{}
	for _, port := range in {
		out = append(out, PortDescription{Name: port.Name, Port: int(port.ContainerPort)})
	}
	return out
}

type ServiceTarget struct {
	Name  string            `json:"name"`
	Type  string            `json:"type"`
	Ports []PortDescription `json:"ports,omitempty"`
}

type ServiceEndpoint struct {
	Name   string      `json:"name"`
	Target string      `json:"target"`
	Ports  map[int]int `json:"ports,omitempty"`
}

type ServiceDefinition struct {
	Name      string            `json:"name"`
	Protocol  string            `json:"protocol"`
	Ports     []int             `json:"ports"`
	Endpoints []ServiceEndpoint `json:"endpoints"`
}

type ServiceManager struct {
	cli *client.VanClient
}

func newServiceManager(cli *client.VanClient) *ServiceManager {
	return &ServiceManager{
		cli: cli,
	}
}

func (m *ServiceManager) resolveEndpoints(ctx context.Context, targetName string, selector string, targetPort map[int]int) ([]ServiceEndpoint, error) {
	endpoints := []ServiceEndpoint{}
	pods, err := kube.GetPods(selector, m.cli.Namespace, m.cli.KubeClient)
	if err != nil {
		return endpoints, err
	}
	for _, pod := range pods {
		endpoints = append(endpoints, ServiceEndpoint{Name: pod.ObjectMeta.Name, Target: targetName, Ports: targetPort})
	}
	return endpoints, nil
}

func (m *ServiceManager) asServiceDefinition(def *types.ServiceInterface) (*ServiceDefinition, error) {
	svc := &ServiceDefinition{
		Name:     def.Address,
		Protocol: def.Protocol,
		Ports:    def.Ports,
	}
	ctx := context.Background()
	for _, target := range def.Targets {
		if target.Selector != "" {
			endpoints, err := m.resolveEndpoints(ctx, target.Name, target.Selector, target.TargetPorts)
			if err != nil {
				return svc, err
			}
			svc.Endpoints = append(svc.Endpoints, endpoints...)
		}
	}
	return svc, nil
}

func (m *ServiceManager) getServices() ([]ServiceDefinition, error) {
	services := []ServiceDefinition{}
	definitions, err := m.cli.ServiceInterfaceList(context.Background())
	if err != nil {
		return services, err
	}
	for _, s := range definitions {
		svc, err := m.asServiceDefinition(s)
		if err != nil {
			return services, err
		}
		services = append(services, *svc)
	}
	return services, nil
}

func (m *ServiceManager) getService(name string) (*ServiceDefinition, error) {
	definition, err := m.cli.ServiceInterfaceInspect(context.Background(), name)
	if err != nil {
		return nil, err
	}
	if definition == nil {
		return nil, nil
	}
	return m.asServiceDefinition(definition)
}

func (m *ServiceManager) createService(options *ServiceOptions) error {
	def := &types.ServiceInterface{
		Address:  options.GetServiceName(),
		Protocol: options.GetProtocol(),
		Ports:    options.GetPorts(),
	}
	deducePort := options.DeducePort()
	target, err := kube.GetServiceInterfaceTarget(options.GetTargetType(), options.GetTargetName(), deducePort, m.cli.Namespace, m.cli.KubeClient)
	if err != nil {
		return err
	}
	if deducePort {
		def.Ports = []int{}
		for _, tPort := range target.TargetPorts {
			def.Ports = append(def.Ports, tPort)
		}
		target.TargetPorts = map[int]int{}
	} else {
		target.TargetPorts = options.GetTargetPorts()
	}
	def.AddTarget(target)
	def.Labels = options.Labels
	return m.cli.ServiceInterfaceUpdate(context.Background(), def)
}

func isServiceNotDefined(err error, name string) bool {
	msg := "Service " + name + " not defined"
	return err.Error() == msg
}

func (m *ServiceManager) deleteService(name string) (bool, error) {
	err := m.cli.ServiceInterfaceRemove(context.Background(), name)
	if err != nil {
		if isServiceNotDefined(err, name) {
			return false, nil
		}
		return false, err
	}
	event.Recordf(ServiceManagement, "Deleted service %q", name)
	return true, nil
}

func (m *ServiceManager) getServiceTargets() ([]ServiceTarget, error) {
	targets := []ServiceTarget{}
	deployments, err := m.cli.KubeClient.AppsV1().Deployments(m.cli.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return targets, err
	}
	for _, deployment := range deployments.Items {
		if deployment.ObjectMeta.Name != types.ControllerDeploymentName && deployment.ObjectMeta.Name != types.TransportDeploymentName {
			targets = append(targets, ServiceTarget{
				Name:  deployment.ObjectMeta.Name,
				Type:  "deployment",
				Ports: asPortDescriptions(kube.GetContainerPorts(&deployment.Spec.Template.Spec)),
			})
		}
	}
	statefulsets, err := m.cli.KubeClient.AppsV1().StatefulSets(m.cli.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return targets, err
	}
	for _, statefulset := range statefulsets.Items {
		targets = append(targets, ServiceTarget{
			Name:  statefulset.ObjectMeta.Name,
			Type:  "statefulset",
			Ports: asPortDescriptions(kube.GetContainerPorts(&statefulset.Spec.Template.Spec)),
		})
	}
	return targets, nil
}

type ServiceOptions struct {
	Address     string            `json:"address"`
	Protocol    string            `json:"protocol"`
	Ports       []int             `json:"ports"`
	TargetPorts map[int]int       `json:"targetPorts,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Target      ServiceTarget     `json:"target"`
}

func (o *ServiceOptions) GetTargetName() string {
	return o.Target.Name
}

func (o *ServiceOptions) GetTargetType() string {
	return o.Target.Type
}

func (o *ServiceOptions) GetServiceName() string {
	if o.Address != "" {
		return o.Address
	}
	return o.Target.Name
}

func (o *ServiceOptions) GetProtocol() string {
	if o.Protocol != "" {
		return o.Protocol
	}
	return "tcp"
}

func (o *ServiceOptions) GetPorts() []int {
	if len(o.Ports) > 0 {
		return o.Ports
	}
	tPorts := []int{}
	for _, tPort := range o.TargetPorts {
		tPorts = append(tPorts, tPort)
	}
	return tPorts
}

func (o *ServiceOptions) GetTargetPorts() map[int]int {
	if len(o.Ports) == 0 {
		// in this case the port will have been set to the
		// target port, which does not then need overridden
		return map[int]int{}
	}
	return o.TargetPorts
}

func (o *ServiceOptions) DeducePort() bool {
	return len(o.Ports) == 0 && len(o.TargetPorts) == 0
}

func getServiceOptions(r *http.Request) (*ServiceOptions, error) {
	options := &ServiceOptions{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return options, err
	}
	if len(body) == 0 {
		return options, fmt.Errorf("Target must be specified in request body")
	}
	err = json.Unmarshal(body, options)
	if err != nil {
		return options, err
	}
	if options.Target.Name == "" || options.Target.Type == "" {
		return options, fmt.Errorf("Target must be specified in request body")
	}
	return options, nil
}

func serveServices(m *ServiceManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if r.Method == http.MethodGet {
			if name, ok := vars["name"]; ok {
				service, err := m.getService(name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else if service == nil {
					http.Error(w, "No such service", http.StatusNotFound)
				} else {
					writeJson(service, w)
				}

			} else {
				services, err := m.getServices()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else {
					writeJson(services, w)
				}
			}
		} else if r.Method == http.MethodPost {
			options, err := getServiceOptions(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				err := m.createService(options)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else {
					event.Recordf("Service %s exposed", options.GetServiceName())
				}
			}
		} else if r.Method == http.MethodDelete {
			if name, ok := vars["name"]; ok {
				deleted, err := m.deleteService(name)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else if !deleted {
					http.Error(w, "No such service", http.StatusNotFound)
				} else {
					event.Recordf("Service %s deleted", name)
				}
			} else {
				http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			}
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})
}

func serveTargets(m *ServiceManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			targets, err := m.getServiceTargets()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				writeJson(targets, w)
			}
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})
}
