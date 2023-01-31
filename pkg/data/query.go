package data

type ConsoleData struct {
	Sites    []Site        `json:"sites"`
	Services []interface{} `json:"services"`
}

type SiteQueryData struct {
	Site
	TcpServices  []TcpService  `json:"tcp_services"`
	HttpServices []HttpService `json:"http_services"`
}

// Used for interacting with 0.4.x sites
type LegacySiteInfo struct {
	SiteId    string
	SiteName  string
	Version   string
	Namespace string
	Url       string
}

func (s *Site) AsLegacySiteInfo() *LegacySiteInfo {
	return &LegacySiteInfo{
		SiteId:    s.SiteId,
		SiteName:  s.SiteName,
		Version:   s.Version,
		Namespace: s.Namespace,
		Url:       s.Url,
	}
}

func (c *ConsoleData) Merge(data []SiteQueryData) {
	http := HttpServiceMap{}
	tcp := TcpServiceMap{}
	for _, d := range data {
		c.Sites = append(c.Sites, d.Site)
		http.merge(d.HttpServices)
		tcp.merge(d.TcpServices)
	}
	c.Services = []interface{}{}
	for _, s := range http {
		c.Services = append(c.Services, s)
	}
	for _, s := range tcp {
		c.Services = append(c.Services, s)
	}
}

type NameMapping interface {
	Lookup(name string) string
}

type NullNameMapping struct {
}

func (n *NullNameMapping) Lookup(name string) string {
	return name
}

func NewNullNameMapping() NameMapping {
	return &NullNameMapping{}
}
