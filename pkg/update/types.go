package update

import "github.com/skupperproject/skupper/api/types"

type Priority int

const (
	PriorityHigh Priority = iota
	PriorityCommon
	PriotityLow
)

type Task interface {
	Info() string
	AppliesTo(siteVersion string) bool
	Version() string
	Priority() Priority
	Platforms() []types.Platform
	Run() Result
}

type Result struct {
	Err  error
	Stop bool
}
