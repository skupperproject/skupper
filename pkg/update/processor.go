package update

import (
	"fmt"
	"sort"

	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/utils"
)

var (
	// All tasks must be registered
	tasks []Task
)

// RegisterTask Platform specific tasks that need client
// can be registered by their respective clients here to
// avoid pkg/ package from importing the clients/ package
func RegisterTask(task Task) {
	tasks = append(tasks, task)
}

// Process goes through the registered list of update tasks and
// filters those that applies to the current platform and site version,
// then it runs each of them, aborting the update process if the
// returned result has the Stop flag set to true.
// If any of the tasks mark the
func Process(siteVersion string) error {
	platform := config.GetPlatform()
	var validTasks []Task
	for _, task := range tasks {
		for _, taskPlatform := range task.Platforms() {
			if taskPlatform == platform && task.AppliesTo(siteVersion) {
				validTasks = append(validTasks, task)
			}
		}
	}

	totalTasks := len(validTasks)

	if totalTasks > 0 {
		// sorting update tasks by priority
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
		fmt.Println("Nothing to do")
		return nil
	}

	// running update tasks
	var errors []error
	for i, task := range validTasks {
		fmt.Printf("Task: %d/%d - version: %s - priority: %d - %s\n", i+1, totalTasks, task.Version(), task.Priority(), task.Info())
		result := task.Run()
		if result.Err != nil {
			errors = append(errors, result.Err)
			fmt.Printf("  -> ERROR: %v\n", result.Err)
		}
		if result.Stop {
			fmt.Println("     unable to proceed")
			return fmt.Errorf("unable to proceed due to the following errors (%d): %v", len(errors), errors)
		}
		fmt.Println()
	}

	if len(errors) > 0 {
		errMsg := fmt.Sprintf("Update completed with %d errors", len(errors))
		fmt.Println(errMsg)
		return fmt.Errorf(errMsg)
	}

	fmt.Println("Update completed")
	return nil
}
