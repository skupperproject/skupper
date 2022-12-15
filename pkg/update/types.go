package update

import "github.com/skupperproject/skupper/api/types"

type Priority int

const (
	PriorityHigh Priority = iota
	PriorityCommon
	PriotityLow
)

// Task is an update task that can be filtered and executed by the update process
type Task interface {
	// Info describes the update task
	Info() string
	// AppliesTo determines whether it should be executed or not
	AppliesTo(siteVersion string) bool
	// Version returns the version it has been introduced
	Version() string
	// Priority determines how tasks within the same version will be sorted
	Priority() Priority
	// Platforms contains all platforms this task applies to
	Platforms() []types.Platform
	// Run method is where the update task is done
	Run() Result
}

// Result contains an eventual error and a Stop flag to control the update process
type Result struct {
	Err  error
	Stop bool
}
