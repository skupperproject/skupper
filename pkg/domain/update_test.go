package domain

import (
	"context"
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
)

func TestUpdateProcessor(t *testing.T) {
	tests := []struct {
		name            string
		curVersion      string
		newVersion      string
		tasks           []UpdateTask
		expectedChanges []string
		expectError     bool
	}{
		{
			name:       "all-tasks-and-post-tasks",
			curVersion: "1.4.2",
			newVersion: "1.5.0",
			tasks: []UpdateTask{
				NewMockUpdateTask(
					"create new container",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityNormal,
					&UpdateResult{
						changes: []string{
							"created new container",
						},
						PostTasks: []UpdateTask{
							NewMockUpdateTask(
								"start new container",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityLast,
								&UpdateResult{
									changes: []string{
										"started new container",
									},
								},
							),
							NewMockUpdateTask(
								"create config file",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityFirst,
								&UpdateResult{
									changes: []string{
										"created config file",
									},
								},
							),
							NewMockUpdateTask(
								"modify config file",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityFirst,
								&UpdateResult{
									changes: []string{
										"modified config file",
									},
								},
							),
						},
					},
				),
				NewMockUpdateTask(
					"create new volume",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityFirst,
					&UpdateResult{
						changes: []string{
							"created new volume",
						},
						PostTasks: []UpdateTask{},
					},
				),
				NewMockUpdateTask(
					"this task should not be executed",
					"1.4.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.4.0")
					},
					PriorityFirst,
					&UpdateResult{
						changes: []string{
							"this task was incorrectly executed",
						},
						PostTasks: []UpdateTask{
							NewMockUpdateTask(
								"this post task should not be executed",
								"1.4.0",
								func(siteVersion string) bool {
									return false
								},
								PriorityFirst,
								&UpdateResult{
									changes: []string{
										"this post task was incorrectly executed",
									},
								},
							),
						},
					},
				),
			},
			expectedChanges: []string{
				"created new volume",
				"created new container",
				"created config file",
				"modified config file",
				"started new container",
			},
			expectError: false,
		},
		{
			name:       "second-task-with-warning",
			curVersion: "1.4.2",
			newVersion: "1.5.0",
			tasks: []UpdateTask{
				NewMockUpdateTask(
					"create new container",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityNormal,
					&UpdateResult{
						warnings: []string{
							"container already exists, ignoring",
						},
						changes: []string{
							"created new container",
						},
						PostTasks: []UpdateTask{
							NewMockUpdateTask(
								"start new container",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityLast,
								&UpdateResult{
									changes: []string{
										"started new container",
									},
								},
							),
						},
					},
				),
				NewMockUpdateTask(
					"create new volume",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityFirst,
					&UpdateResult{
						changes: []string{
							"created new volume",
						},
						PostTasks: []UpdateTask{},
					},
				),
			},
			expectedChanges: []string{
				"created new volume",
				"created new container",
				"started new container",
			},
			expectError: false,
		},
		{
			name:       "second-task-with-error",
			curVersion: "1.4.2",
			newVersion: "1.5.0",
			tasks: []UpdateTask{
				NewMockUpdateTask(
					"create new container",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityNormal,
					&UpdateResult{
						Errors: []error{
							fmt.Errorf("container already exists"),
						},
						PostTasks: []UpdateTask{
							NewMockUpdateTask(
								"start new container",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityLast,
								&UpdateResult{
									changes: []string{
										"should not be executed - started new container",
									},
								},
							),
						},
					},
				),
				NewMockUpdateTask(
					"create new volume",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityFirst,
					&UpdateResult{
						changes: []string{
							"created new volume",
						},
						PostTasks: []UpdateTask{},
					},
				),
			},
			expectedChanges: []string{
				"created new volume",
			},
			expectError: true,
		},
		{
			name:       "second-post-task-with-error",
			curVersion: "1.4.2",
			newVersion: "1.5.0",
			tasks: []UpdateTask{
				NewMockUpdateTask(
					"create new container",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityNormal,
					&UpdateResult{
						changes: []string{
							"created new container",
						},
						PostTasks: []UpdateTask{
							NewMockUpdateTask(
								"start new container",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityLast,
								&UpdateResult{
									changes: []string{
										"started new container",
									},
								},
							),
							NewMockUpdateTask(
								"create config file",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityFirst,
								&UpdateResult{
									changes: []string{
										"created config file",
									},
								},
							),
							NewMockUpdateTask(
								"modify config file",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityFirst,
								&UpdateResult{
									Errors: []error{
										fmt.Errorf("error modifying file"),
									},
								},
							),
						},
					},
				),
				NewMockUpdateTask(
					"create new volume",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityFirst,
					&UpdateResult{
						changes: []string{
							"created new volume",
						},
						PostTasks: []UpdateTask{},
					},
				),
				NewMockUpdateTask(
					"this task should not be executed",
					"1.4.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.4.0")
					},
					PriorityFirst,
					&UpdateResult{
						changes: []string{
							"this task was incorrectly executed",
						},
						PostTasks: []UpdateTask{
							NewMockUpdateTask(
								"this post task should not be executed",
								"1.4.0",
								func(siteVersion string) bool {
									return false
								},
								PriorityFirst,
								&UpdateResult{
									changes: []string{
										"this post task was incorrectly executed",
									},
								},
							),
						},
					},
				),
			},
			expectedChanges: []string{
				"created new volume",
				"created new container",
				"created config file",
			},
			expectError: true,
		},
		{
			name:       "no-updated-needed",
			curVersion: "1.5.0",
			newVersion: "1.5.0",
			tasks: []UpdateTask{
				NewMockUpdateTask(
					"create new container",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityNormal,
					&UpdateResult{
						changes: []string{
							"created new container",
						},
						PostTasks: []UpdateTask{
							NewMockUpdateTask(
								"start new container",
								"1.5.0",
								func(siteVersion string) bool {
									return true
								},
								PriorityLast,
								&UpdateResult{
									changes: []string{
										"started new container",
									},
								},
							),
						},
					},
				),
				NewMockUpdateTask(
					"create new volume",
					"1.5.0",
					func(siteVersion string) bool {
						return utils.LessRecentThanVersion(siteVersion, "1.5.0")
					},
					PriorityFirst,
					&UpdateResult{
						changes: []string{
							"created new volume",
						},
						PostTasks: []UpdateTask{},
					},
				),
			},
			expectError: false,
		},
	}

	for _, scenario := range tests {
		t.Run(scenario.name, func(t *testing.T) {
			up := &UpdateProcessor{
				Verbose: true,
			}
			up.RegisterTasks(scenario.tasks...)
			err := up.Process(context.Background(), scenario.curVersion)
			assert.Equal(t, err != nil, scenario.expectError, fmt.Sprintf("error expected == %v but got err == %v", scenario.expectError, err))
			assert.DeepEqual(t, scenario.expectedChanges, up.changes)
		})
	}
}

func NewMockUpdateTask(info, version string, appliesTo func(siteVersion string) bool, priority UpdatePriority, result *UpdateResult) UpdateTask {
	return &mockUpdateTask{
		info:      info,
		version:   version,
		appliesTo: appliesTo,
		priority:  priority,
		result:    result,
	}
}

type mockUpdateTask struct {
	info      string
	version   string
	appliesTo func(siteVersion string) bool
	priority  UpdatePriority
	result    *UpdateResult
}

func (u *mockUpdateTask) Info() string {
	return u.info
}

func (u *mockUpdateTask) AppliesTo(siteVersion string) bool {
	return u.appliesTo(siteVersion)
}

func (u *mockUpdateTask) Version() string {
	return u.version
}

func (u *mockUpdateTask) Priority() UpdatePriority {
	return u.priority
}

func (u *mockUpdateTask) Run(context.Context) *UpdateResult {
	return u.result
}
