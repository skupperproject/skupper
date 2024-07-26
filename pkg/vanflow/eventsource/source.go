package eventsource

import (
	"time"
)

// Info describes a vanflow event source
type Info struct {
	ID       string
	Version  int
	Type     string
	Address  string
	Direct   string
	LastSeen time.Time
}
