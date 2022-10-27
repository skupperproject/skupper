package domain

type SiteIngress interface {
	GetName() string
	GetHost() string
	GetPort() int
	GetTarget() Port
}

// SiteIngressCommon Partial implementation of SiteIngress
type SiteIngressCommon struct {
	Name   string
	Host   string
	Port   int
	Target *PortCommon
}

func (s *SiteIngressCommon) GetName() string {
	return s.Name
}

func (s *SiteIngressCommon) GetHost() string {
	return s.Host
}

func (s *SiteIngressCommon) GetPort() int {
	return s.Port
}

func (s *SiteIngressCommon) GetTarget() Port {
	return s.Target
}

type AddressIngress interface {
	GetHost() string
	SetHost(host string)
	GetPorts() map[int]int
	SetPorts(ports map[int]int)
}

type AddressIngressCommon struct {
	Host  string
	Ports map[int]int
}

func (a *AddressIngressCommon) GetHost() string {
	return a.Host
}

func (a *AddressIngressCommon) SetHost(host string) {
	a.Host = host
}

func (a *AddressIngressCommon) GetPorts() map[int]int {
	return a.Ports
}

func (a *AddressIngressCommon) SetPorts(ports map[int]int) {
	a.Ports = ports
}

type Port interface {
	GetName() string
	GetPort() int
}

type PortCommon struct {
	Name string
	Port int
}

func (p *PortCommon) GetName() string {
	return p.Name
}

func (p *PortCommon) GetPort() int {
	return p.Port
}
