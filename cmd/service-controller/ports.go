package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/pkg/qdr"
)

type PortRange struct {
	Start int
	End   int
}

type FreePorts struct {
	Available []PortRange
}

const (
	MIN_PORT = 1024
	MAX_PORT = 65535
)

func newFreePorts() *FreePorts {
	return &FreePorts{
		Available: []PortRange{
			PortRange{
				Start: MIN_PORT,
				End:   MAX_PORT,
			},
		},
	}
}

func (ports *PortRange) extend(port int) bool {
	if port == ports.Start-1 {
		ports.Start = port
		return true
	} else if port == ports.End+1 {
		ports.End = port
		return true
	} else {
		return false
	}
}

func (ports *PortRange) size() int {
	return ports.End - ports.Start + 1
}

func (ports PortRange) String() string {
	return fmt.Sprintf("(%d-%d)", ports.Start, ports.End)
}

func (ports *PortRange) merge(other PortRange) bool {
	if ports.contains(other.Start) && ports.contains(other.End) {
		//other wholly contained within ports
		return true
	} else if other.contains(ports.Start) && other.contains(ports.End) {
		//ports wholly contained within other
		ports.Start = other.Start
		ports.End = other.End
		return true
	} else if other.Start > ports.End+1 || other.End+1 < ports.Start {
		//no overlap
		return false
	} else if other.Start == (ports.End + 1) {
		//adjacent, port precedes other
		ports.End = other.End
		return true
	} else if (other.End + 1) == ports.Start {
		//adjacent, other precedes ports
		ports.Start = other.Start
		return true
	} else if ports.Start <= other.Start && ports.End >= other.Start {
		//overlap, ports precedes other
		ports.End = other.End
		return true
	} else if other.Start <= ports.Start && other.End >= ports.Start {
		//overlap, other precedes ports
		ports.Start = other.Start
		return true
	} else {
		return false
	}
}

func (ports *PortRange) contains(port int) bool {
	return port <= ports.End && port >= ports.Start
}

func removePortRange(ports []PortRange, i int) []PortRange {
	if len(ports) > 1 {
		if i == len(ports)-1 {
			return ports[:i]
		} else if i == 0 {
			return ports[1:]
		} else {
			return append(ports[:i], ports[i+1:]...)
		}
	} else if i == 0 {
		return []PortRange{}
	} else {
		return ports
	}
}

func insertPortRange(ports []PortRange, extra PortRange, i int) []PortRange {
	if i == 0 {
		return append([]PortRange{extra}, ports...)
	} else if i+1 > len(ports) {
		return append(ports, extra)
	} else {
		copy := []PortRange{}
		for index, v := range ports {
			if index == i {
				copy = append(copy, extra)
			}
			copy = append(copy, v)
		}
		return copy
	}
}

func (ports *FreePorts) String() string {
	parts := []string{}
	for _, r := range ports.Available {
		parts = append(parts, r.String())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (ports *FreePorts) release(port int) bool {
	var i int
	for i = 0; i < len(ports.Available) && port >= (ports.Available[i].Start-1); i++ {
		if ports.Available[i].contains(port) {
			return false
		} else if ports.Available[i].extend(port) {
			if i+1 < len(ports.Available) && ports.Available[i].merge(ports.Available[i+1]) {
				//need to remove ports.Available[i+1]
				ports.Available = removePortRange(ports.Available, i+1)
			}
			return true
		}
	}
	//need to append a new range
	extra := PortRange{
		Start: port,
		End:   port,
	}
	ports.Available = insertPortRange(ports.Available, extra, i)
	return true
}

func (ports *FreePorts) inuse(port int) bool {
	for i := 0; i < len(ports.Available) && port >= ports.Available[i].Start; i++ {
		if ports.Available[i].contains(port) {
			if port == ports.Available[i].Start {
				if ports.Available[i].size() == 1 {
					//remove it entirely
					ports.Available = removePortRange(ports.Available, i)
				} else {
					ports.Available[i].Start++
				}
			} else if port == ports.Available[i].End {
				//size must be greater than 1 or clause above would have matched
				ports.Available[i].End--
			} else {
				//need to split the range
				extra := PortRange{
					Start: port + 1,
					End:   ports.Available[i].End,
				}
				ports.Available[i].End = port - 1
				ports.Available = insertPortRange(ports.Available, extra, i+1)
			}
			return true
		}
	}
	return false
}

func (ports *FreePorts) nextFreePort() (int, error) {
	if len(ports.Available) > 0 {
		next := ports.Available[0].Start
		ports.inuse(next)
		return next, nil
	} else {
		return 0, fmt.Errorf("No available ports")
	}
}

func portAsInt(port string) int {
	result, _ := strconv.Atoi(port)
	return result
}

func (ports *FreePorts) getPortAllocations(bridges *qdr.BridgeConfig) map[string][]int {
	allocations := map[string][]int{}
	addPort := func(address string, port int) {
		if curPorts, found := allocations[address]; !found {
			allocations[address] = []int{port}
		} else {
			curPorts = append(curPorts, port)
			allocations[address] = curPorts
		}
	}
	if bridges != nil {
		for _, b := range bridges.HttpConnectors {
			port := portAsInt(b.Port)
			ports.inuse(port)
		}
		for _, b := range bridges.HttpListeners {
			address := strings.Split(b.Address, ":")[0]
			port := portAsInt(b.Port)
			addPort(address, port)
			ports.inuse(port)
		}
		for _, b := range bridges.TcpConnectors {
			port := portAsInt(b.Port)
			ports.inuse(port)
		}
		for _, b := range bridges.TcpListeners {
			address := strings.Split(b.Address, ":")[0]
			port := portAsInt(b.Port)
			addPort(address, port)
			ports.inuse(port)
		}
	}
	return allocations
}
