package network

type NetworkStatusInfo struct {
	Addresses  []AddressInfo    `json:"addresses"`
	SiteStatus []SiteStatusInfo `json:"siteStatus"`
}

type AddressInfo struct {
	RecType        string `json:"recType,omitempty"`
	Identity       string `json:"identity,omitempty"`
	Name           string `json:"name,omitempty"`
	StartTime      uint64 `json:"startTime"`
	EndTime        uint64 `json:"endTime"`
	Protocol       string `json:"protocol,omitempty"`
	ListenerCount  int    `json:"listenerCount"`
	ConnectorCount int    `json:"connectorCount"`
}

type SiteStatusInfo struct {
	Site         SiteInfo           `json:"site"`
	RouterStatus []RouterStatusInfo `json:"routerStatus"`
}

type SiteInfo struct {
	Identity       string `json:"identity,omitempty"`
	Name           string `json:"name,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	Platform       string `json:"platform,omitempty"`
	Version        string `json:"siteVersion,omitempty"`
	MinimumVersion string `json:"minimumVersion,omitempty"`
	Policy         string `json:"policy,omitempty"`
}

type RouterStatusInfo struct {
	Router     RouterInfo      `json:"router"`
	Links      []LinkInfo      `json:"links"`
	Listeners  []ListenerInfo  `json:"listeners"`
	Connectors []ConnectorInfo `json:"connectors"`
}

type RouterInfo struct {
	Name         string `json:"name,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	Mode         string `json:"mode,omitempty"`
	ImageName    string `json:"imageName,omitempty"`
	ImageVersion string `json:"imageVersion,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
}

type LinkInfo struct {
	Mode      string `json:"mode,omitempty"`
	Name      string `json:"name,omitempty"`
	LinkCost  uint64 `json:"linkCost,omitempty"`
	Direction string `json:"direction,omitempty"`
}

type ListenerInfo struct {
	Name     string `json:"name,omitempty"`
	DestHost string `json:"destHost,omitempty"`
	DestPort string `json:"destPort,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Address  string `json:"address,omitempty"`
}

type ConnectorInfo struct {
	DestHost string `json:"destHost,omitempty"`
	DestPort string `json:"destPort,omitempty"`
	Address  string `json:"address,omitempty"`
	Process  string `json:"process,omitempty"`
	Target   string `json:"target,omitempty"`
}

type SiteInfoForLinks struct {
	Name           string   `json:"site_name,omitempty"`
	Namespace      string   `json:"namespace,omitempty"`
	SiteId         string   `json:"site_id,omitempty"`
	Platform       string   `json:"platform,omitempty"`
	Url            string   `json:"url,omitempty"`
	Version        string   `json:"version,omitempty"`
	Gateway        bool     `json:"gateway,omitempty"`
	MinimumVersion string   `json:"minimum_version,omitempty"`
	Links          []string `json:"connected,omitempty"`
}

type RemoteLinkInfo struct {
	SiteName  string `json:"siteName,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	SiteId    string `json:"siteId,omitempty"`
	LinkName  string `json:"linkName,omitempty"`
}

type LocalSiteInfo struct {
	SiteId      string
	ServiceInfo map[string]LocalServiceInfo
}
type LocalServiceInfo struct {
	Data map[string][]string
}
