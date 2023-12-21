package domain

import "context"

type SkupperDeployment interface {
	GetName() string
	GetComponents() []SkupperComponent
}

type SkupperDeploymentHandler interface {
	Deploy(ctx context.Context, deployment SkupperDeployment) error
	List() ([]SkupperDeployment, error)
	Undeploy(name string) error
}

type SkupperDeploymentCommon struct {
	Components []SkupperComponent
}

func (s *SkupperDeploymentCommon) GetComponents() []SkupperComponent {
	return s.Components
}
