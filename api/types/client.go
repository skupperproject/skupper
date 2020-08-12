package types

type ConnectorCreateOptions struct {
	SkupperNamespace string
	Name             string
	Cost             int32
}

type ConnectorRemoveOptions struct {
	SkupperNamespace string
	Name             string
	ForceCurrent     bool
}

type ConnectorInspectResponse struct {
	SkupperNamespace string
	Connector        *Connector
	Connected        bool
}

type SiteConfig struct {
	Spec      SiteConfigSpec
	Reference SiteConfigReference
}

type SiteConfigSpec struct {
	SkupperName         string
	SkupperNamespace    string
	IsEdge              bool
	EnableController    bool
	EnableServiceSync   bool
	EnableRouterConsole bool
	EnableConsole       bool
	AuthMode            string
	User                string
	Password            string
	ClusterLocal        bool
	Replicas            int32
	SiteControlled      bool
}

type SiteConfigReference struct {
	UID        string
	Name       string
	APIVersion string
	Kind       string
}

type ServiceInterfaceCreateOptions struct {
	Protocol   string
	Address    string
	Port       int
	TargetPort int
	Headless   bool
}

type RouterInspectResponse struct {
	Status            RouterStatusSpec
	TransportVersion  string
	ControllerVersion string
	ExposedServices   int
}
