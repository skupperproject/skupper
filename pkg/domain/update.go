package domain

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/skupperproject/skupper/pkg/utils"
)

type UpdatePriority int

const (
	PriorityFirst UpdatePriority = iota
	PriorityNormal
	PriorityLast
)

// UpdateResult contains errors, warnings and changes applied by tasks.
// It also contains a set of PostTasks that can be populated by tasks.
// The PostTasks will only be evaluated after all main tasks complete.
type UpdateResult struct {
	Errors    []error
	warnings  []string
	changes   []string
	PostTasks []UpdateTask
}

func (u *UpdateResult) AddErrors(err ...error) {
	u.Errors = append(u.Errors, err...)
}

func (u *UpdateResult) AddWarnings(msg ...string) {
	u.warnings = append(u.warnings, msg...)
}

func (u *UpdateResult) GetWarnings() []string {
	return u.warnings
}

func (u *UpdateResult) Changed() bool {
	return len(u.changes) > 0
}

func (u *UpdateResult) AddChange(change ...string) {
	u.changes = append(u.changes, change...)
}

func (u *UpdateResult) GetChanges() []string {
	return u.changes
}

type UpdateTask interface {
	// Info describes the update task
	Info() string
	// AppliesTo determines whether it should be executed or not
	AppliesTo(siteVersion string) bool
	// Version returns the version it has been introduced (use * when not constrained to a given version)
	Version() string
	// Priority determines how tasks will be sorted by the UpdateProcessor
	Priority() UpdatePriority
	// Run method is where the update task is done
	Run() *UpdateResult
}

type UpdateProcessor struct {
	tasks     []UpdateTask
	postTasks []UpdateTask
	changes   []string
	Verbose   bool
	DryRun    bool
}

func (p *UpdateProcessor) Println(a ...any) {
	if p.Verbose {
		fmt.Println(a...)
	}
}

func (p *UpdateProcessor) Printf(format string, a ...any) {
	if p.Verbose {
		fmt.Printf(format, a...)
	}
}

// RegisterTasks registers tasks to be evaluated and processed
func (p *UpdateProcessor) RegisterTasks(task ...UpdateTask) {
	p.tasks = append(p.tasks, task...)
}

// registerPostTasks register post tasks to be evaluated and processed
// after all tasks have completed.
// The Post Tasks are processed only once.
func (p *UpdateProcessor) registerPostTasks(tasks ...UpdateTask) {
	for _, task := range tasks {
		add := true
		for _, postTask := range p.postTasks {
			if task == postTask {
				add = false
				break
			}
		}
		if add {
			p.postTasks = append(p.postTasks, task)
		}
	}
}

func (p *UpdateProcessor) Process(siteVersion string) error {
	err := p.process(siteVersion, p.tasks)
	if err != nil {
		return err
	}
	if len(p.postTasks) > 0 {
		p.Println("Post update tasks")
		return p.process(siteVersion, p.postTasks)
	}
	if len(p.changes) > 0 {
		fmt.Println("Skupper is now updated for '" + utils.ReadUsername() + "'.")
	} else {
		fmt.Println("No update required for '" + utils.ReadUsername() + "'.")
	}
	return nil
}

func (p *UpdateProcessor) process(siteVersion string, tasks []UpdateTask) error {
	var validTasks []UpdateTask
	title := "Task"
	postTasks := false
	if reflect.DeepEqual(tasks, p.postTasks) {
		title = "Post task"
		postTasks = true
	}
	for _, task := range tasks {
		if task.AppliesTo(siteVersion) {
			validTasks = append(validTasks, task)
		}
	}
	totalTasks := len(validTasks)

	if totalTasks > 0 {
		// sorting update tasks by version and priority
		sort.SliceStable(validTasks, func(i, j int) bool {
			t1 := validTasks[i]
			t2 := validTasks[j]
			sameVersion := utils.EquivalentVersion(t1.Version(), t2.Version())
			if !sameVersion && (t1.Version() != "*" && t2.Version() != "*") {
				return utils.LessRecentThanVersion(t1.Version(), t2.Version())
			} else if t1.Version() == "*" {
				return false
			} else if t2.Version() == "*" {
				return true
			}
			return t1.Priority() < t2.Priority()
		})
	} else {
		p.Println("Nothing to do")
		return nil
	}

	// running update tasks
	var errors []error
	var warnings []string
	for i, task := range validTasks {
		version := task.Version()
		if version == "*" {
			version = "all"
		}
		p.Printf("%s: %d/%d - version: %s - priority: %d - %s\n",
			title, i+1, totalTasks, version, task.Priority(), task.Info())
		var result *UpdateResult
		if !p.DryRun {
			result = task.Run()
			if result == nil {
				result = &UpdateResult{}
			}
		} else {
			result = &UpdateResult{}
		}
		if len(result.warnings) > 0 {
			warnings = append(warnings, result.warnings...)
			p.Printf("  -> WARNING: %v\n", result.warnings)
		}
		if len(result.Errors) > 0 {
			errors = append(errors, result.Errors...)
			p.Printf("  -> ERROR: %v\n", result.Errors)
			p.Println("     unable to proceed")
			return fmt.Errorf("unable to proceed due to the following errors (%d): %v", len(errors), errors)
		}
		// post tasks are only processed once, for main tasks
		if !postTasks && len(result.PostTasks) > 0 {
			p.registerPostTasks(result.PostTasks...)
		}
		if len(result.GetChanges()) > 0 {
			p.changes = append(p.changes, result.GetChanges()...)
		}
		p.Println()
	}

	// warnings are considered as informative only and will not
	// cause the update command to fail (rc != 0)
	if !postTasks && len(warnings) > 0 {
		errMsg := fmt.Sprintf("Update completed with (%d) warnings: %v", len(warnings), warnings)
		p.Println(errMsg)
	}

	return nil
}
