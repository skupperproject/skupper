package types

type VanConnectorCreateOptions struct {
	Name string
	Cost int32
}

type VanRouterCreateOptions struct {
	SkupperName       string
	IsEdge            bool
	EnableController  bool
	EnableServiceSync bool
	EnableConsole     bool
	AuthMode          string
	User              string
	Password          string
	ClusterLocal      bool
	Replicas          int32
}

type VanServiceInterfaceCreateOptions struct {
	Protocol   string
	Address    string
	Port       int
	TargetPort int
	Headless   bool
}

type VanRouterInspectResponse struct {
	Status            VanRouterStatusSpec
	QdrVersion        string
	ControllerVersion string
}

type VanConnectorInspectResponse struct {
	Connector *Connector
	Connected bool
}
