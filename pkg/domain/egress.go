package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

type EgressResolver interface {
	// String returns an expression representing selectable targets
	String() string
	Resolve() ([]AddressEgress, error)
}

type AddressEgress interface {
	GetHost() string
	SetHost(host string)
	GetPorts() map[int]int
	SetPorts(ports map[int]int)
}

type AddressEgressCommon struct {
	Host  string
	Ports map[int]int
}

func (a *AddressEgressCommon) GetHost() string {
	return a.Host
}

func (a *AddressEgressCommon) SetHost(host string) {
	a.Host = host
}

func (a *AddressEgressCommon) GetPorts() map[int]int {
	return a.Ports
}

func (a *AddressEgressCommon) SetPorts(ports map[int]int) {
	a.Ports = ports
}

//
// Egress Resolvers
//

// EgressResolverFromString returns an EgressResolver based
// on provided string representation
func EgressResolverFromString(data string) EgressResolver {
	if data == "" || !strings.Contains(data, "=") {
		return nil
	}
	typeJson := strings.SplitN(data, "=", 2)
	resolverType := typeJson[0]
	jsonData := typeJson[1]

	var resolver EgressResolver
	switch resolverType {
	case "*domain.EgressResolverHost":
		resolver = &EgressResolverHost{}
		_ = json.Unmarshal([]byte(jsonData), resolver)
		return resolver
	}
	return nil
}

type EgressResolverHost struct {
	Host  string      `json:"host"`
	Ports map[int]int `json:"ports"`
}

func (e *EgressResolverHost) String() string {
	jsonData, _ := json.Marshal(e)
	return fmt.Sprintf("%T=%s", e, string(jsonData))
}

func (e *EgressResolverHost) Resolve() ([]AddressEgress, error) {
	return []AddressEgress{
		&AddressEgressCommon{
			Host:  e.Host,
			Ports: e.Ports,
		},
	}, nil
}
