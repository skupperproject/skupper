package domain

type SkupperDeployment interface {
	GetName() string
	GetComponents() []SkupperComponent
}

type SkupperDeploymentHandler interface {
	Deploy(deployment SkupperDeployment) error
	List() ([]SkupperDeployment, error)
	Undeploy(name string) error
}

type SkupperDeploymentCommon struct {
	Components []SkupperComponent
}

func (s *SkupperDeploymentCommon) GetComponents() []SkupperComponent {
	return s.Components
}
