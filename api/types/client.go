package types

type VanConnectorCreateOptions struct {
	SkupperNamespace string
	Name             string
	Cost             int32
}

type VanConnectorRemoveOptions struct {
	SkupperNamespace string
	Name             string
	ForceCurrent     bool
}

type VanConnectorInspectResponse struct {
	SkupperNamespace string
	Connector        *Connector
	Connected        bool
}

type VanSiteConfig struct {
	Spec      VanSiteConfigSpec
	Reference VanSiteConfigReference
}

type VanSiteConfigSpec struct {
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

type VanSiteConfigReference struct {
	UID        string
	Name       string
	APIVersion string
	Kind       string
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
	TransportVersion  string
	ControllerVersion string
	ExposedServices   int
}
