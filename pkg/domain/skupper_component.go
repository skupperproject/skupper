package domain

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/test/utils"
)

type SkupperComponent interface {
	Name() string
	GetImage() string
	SetImage(image string)
	GetEnv() map[string]string
	GetLabels() map[string]string
	GetSiteIngresses() []SiteIngress
	GetMemoryLimit() int64
	GetCpus() int
}

type SkupperComponentHandler interface {
	Get(name string) (SkupperComponent, error)
	List() ([]SkupperComponent, error)
}

type Router struct {
	Image         string
	Env           map[string]string
	Labels        map[string]string
	SiteIngresses []SiteIngress
	MemoryLimit   int64
	Cpus          int
}

func (r *Router) Name() string {
	return types.TransportDeploymentName
}

func (r *Router) GetImage() string {
	return utils.StrDefault(images.GetRouterImageName(), r.Image)
}

func (r *Router) SetImage(image string) {
	r.Image = image
}

func (r *Router) GetEnv() map[string]string {
	if r.Env == nil {
		r.Env = map[string]string{}
	}
	return r.Env
}

func (r *Router) GetLabels() map[string]string {
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	return r.Labels
}

func (r *Router) GetSiteIngresses() []SiteIngress {
	return r.SiteIngresses
}

func (r *Router) GetMemoryLimit() int64 {
	return r.MemoryLimit
}

func (r *Router) GetCpus() int {
	return r.Cpus
}

type FlowCollector struct {
	Image         string
	Env           map[string]string
	Labels        map[string]string
	SiteIngresses []SiteIngress
	MemoryLimit   int64
	Cpus          int
}

func (r *FlowCollector) Name() string {
	return types.FlowCollectorContainerName
}

func (r *FlowCollector) GetImage() string {
	return utils.StrDefault(images.GetFlowCollectorImageName(), r.Image)
}

func (r *FlowCollector) SetImage(image string) {
	r.Image = image
}

func (r *FlowCollector) GetEnv() map[string]string {
	if r.Env == nil {
		r.Env = map[string]string{}
	}
	return r.Env
}

func (r *FlowCollector) GetLabels() map[string]string {
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	return r.Labels
}

func (r *FlowCollector) GetSiteIngresses() []SiteIngress {
	return r.SiteIngresses
}

func (r *FlowCollector) GetMemoryLimit() int64 {
	return r.MemoryLimit
}

func (r *FlowCollector) GetCpus() int {
	return r.Cpus
}

type Controller struct {
	Image         string
	Env           map[string]string
	Labels        map[string]string
	SiteIngresses []SiteIngress
	MemoryLimit   int64
	Cpus          int
}

func (s *Controller) Name() string {
	return types.ControllerPodmanContainerName
}

func (s *Controller) GetImage() string {
	return utils.StrDefault(images.GetControllerPodmanImageName(), s.Image)
}

func (s *Controller) SetImage(image string) {
	s.Image = image
}

func (s *Controller) GetEnv() map[string]string {
	if s.Env == nil {
		s.Env = map[string]string{}
	}
	return s.Env
}

func (s *Controller) GetLabels() map[string]string {
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s.Labels
}

func (s *Controller) GetSiteIngresses() []SiteIngress {
	return s.SiteIngresses
}

func (s *Controller) GetMemoryLimit() int64 {
	return s.MemoryLimit
}

func (s *Controller) GetCpus() int {
	return s.Cpus
}

type Prometheus struct {
	Image         string
	Env           map[string]string
	Labels        map[string]string
	SiteIngresses []SiteIngress
	MemoryLimit   int64
	Cpus          int
}

func (s *Prometheus) Name() string {
	return types.PrometheusDeploymentName
}

func (s *Prometheus) GetImage() string {
	return images.GetPrometheusServerImageName()
}

func (s *Prometheus) SetImage(image string) {
	s.Image = image
}

func (s *Prometheus) GetEnv() map[string]string {
	if s.Env == nil {
		s.Env = map[string]string{}
	}
	return s.Env
}

func (s *Prometheus) GetLabels() map[string]string {
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s.Labels
}

func (s *Prometheus) GetSiteIngresses() []SiteIngress {
	return s.SiteIngresses
}

func (s *Prometheus) GetMemoryLimit() int64 {
	return s.MemoryLimit
}

func (s *Prometheus) GetCpus() int {
	return s.Cpus
}
