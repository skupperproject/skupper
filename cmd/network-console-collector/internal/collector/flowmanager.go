package collector

type pair struct {
	Source   string
	Dest     string
	Protocol string
}
type processAttributes struct {
	ID       string
	Name     string
	SiteID   string
	SiteName string
}

type connectorAttrs struct {
	Protocol string
	Address  string
}
