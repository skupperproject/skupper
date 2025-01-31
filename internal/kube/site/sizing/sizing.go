package sizing

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

const (
	SiteSizingLabel             = "skupper.io/site-sizing"
	DefaultSiteSizingAnnotation = "skupper.io/default-site-sizing"
)

type Registry struct {
	sizes       map[string]*corev1.ConfigMap //keyed on size name
	names       map[string]string            //ConfigMap key -> size name
	defaultSize string
}

func NewRegistry() *Registry {
	return &Registry{
		sizes: map[string]*corev1.ConfigMap{},
		names: map[string]string{},
	}
}

func (r *Registry) Update(key string, cm *corev1.ConfigMap) error {
	if name, ok := getSizeName(cm); ok {
		if existing, ok := r.names[key]; ok {
			delete(r.sizes, existing)
		}
		r.names[key] = name
		r.sizes[name] = cm
		if isDefault(cm) {
			r.defaultSize = name
		}
	} else {
		if name, ok := r.names[key]; ok {
			delete(r.names, key)
			delete(r.sizes, name)
		}
	}
	return nil
}

func getSizeName(cm *corev1.ConfigMap) (string, bool) {
	if cm == nil || cm.ObjectMeta.Labels == nil {
		return "", false
	}
	name, ok := cm.ObjectMeta.Labels[SiteSizingLabel]
	return name, ok
}

func isDefault(cm *corev1.ConfigMap) bool {
	if cm == nil || cm.ObjectMeta.Annotations == nil {
		return false
	}
	_, ok := cm.ObjectMeta.Annotations[DefaultSiteSizingAnnotation]
	return ok
}

func (r *Registry) GetSizing(site *skupperv2alpha1.Site) (Sizing, error) {
	if config := r.getSizeConfiguration(desiredSize(site)); config != nil {
		return parse(config)
	}
	return Sizing{}, nil
}

func desiredSize(site *skupperv2alpha1.Site) string {
	if site.Spec.Settings == nil {
		return ""
	}
	return site.Spec.Settings["size"]
}

func (r *Registry) getSizeConfiguration(name string) *corev1.ConfigMap {
	if conf, ok := r.sizes[name]; ok {
		return conf
	}
	return r.sizes[r.defaultSize]
}

type Sizing struct {
	Router  ContainerResources
	Adaptor ContainerResources
}

func parse(cm *corev1.ConfigMap) (Sizing, error) {
	var errs []error
	sizing := Sizing{
		Router: ContainerResources{
			Requests: map[string]string{},
			Limits:   map[string]string{},
		},
		Adaptor: ContainerResources{
			Requests: map[string]string{},
			Limits:   map[string]string{},
		},
	}
	for key, value := range cm.Data {
		if err := verify(value); err != nil {
			errs = append(errs, fmt.Errorf("Bad value for %s in %s/%s: %s", key, cm.Namespace, cm.Name, err))
			continue
		}
		switch key {
		case "router-cpu-request":
			sizing.Router.setCpuRequest(value)
		case "router-cpu-limit":
			sizing.Router.setCpuLimit(value)
		case "router-memory-request":
			sizing.Router.setMemoryRequest(value)
		case "router-memory-limit":
			sizing.Router.setMemoryLimit(value)
		case "adaptor-cpu-request":
			sizing.Adaptor.setCpuRequest(value)
		case "adaptor-cpu-limit":
			sizing.Adaptor.setCpuLimit(value)
		case "adaptor-memory-request":
			sizing.Adaptor.setMemoryRequest(value)
		case "adaptor-memory-limit":
			sizing.Adaptor.setMemoryLimit(value)
		default:
			errs = append(errs, fmt.Errorf("Ignoring key %s in %s/%s", key, cm.Namespace, cm.Name))
		}
	}
	return sizing, errors.Join(errs...)
}

type ContainerResources struct {
	Requests map[string]string
	Limits   map[string]string
}

func (r ContainerResources) NotEmpty() bool {
	return len(r.Requests) > 0 || len(r.Limits) > 0
}

func (r *ContainerResources) setCpuRequest(value string) {
	r.Requests[string(corev1.ResourceCPU)] = value
}

func (r *ContainerResources) setCpuLimit(value string) {
	r.Limits[string(corev1.ResourceCPU)] = value
}

func (r *ContainerResources) setMemoryRequest(value string) {
	r.Requests[string(corev1.ResourceMemory)] = value
}

func (r *ContainerResources) setMemoryLimit(value string) {
	r.Limits[string(corev1.ResourceMemory)] = value
}

func verify(value string) error {
	_, err := resource.ParseQuantity(value)
	return err
}

//How is change in sizing handled? Need to reconcile every site (could wait until the next site timeout event...)
