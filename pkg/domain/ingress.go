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
	Address() string
	Host() string
	Port() int
	Protocol() string
}

type EgressResolver interface {
	Name() string
	Resolve() []AddressEgress
}

type AddressEgress interface {
	Address() string
	Host() string
	Port() int
	Protocol() string
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
