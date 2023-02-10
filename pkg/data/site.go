package data

type Site struct {
	SiteName  string   `json:"site_name"`
	SiteId    string   `json:"site_id"`
	Version   string   `json:"version"`
	Platform  string   `json:"platform"`
	Connected []string `json:"connected"`
	Namespace string   `json:"namespace"`
	Url       string   `json:"url"`
	Edge      bool     `json:"edge"`
	Gateway   bool     `json:"gateway"`
}

func (s *Site) IsConnectedTo(siteId string) bool {
	for _, value := range s.Connected {
		if value == siteId {
			return true
		}
	}
	return false
}
