package domain

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/images"
)

type SkupperComponent interface {
	Name() string
	GetImage() string
	SetImage(image string)
	GetEnv() map[string]string
	GetLabels() map[string]string
	GetSiteIngresses() []SiteIngress
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
}

func (r *Router) Name() string {
	return types.TransportDeploymentName
}

func (r *Router) GetImage() string {
	return images.GetRouterImageName()
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
