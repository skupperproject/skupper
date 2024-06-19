package podman

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
)

func TestContainerInformer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cli, wg := NewClientOrSkip(t, ctx)
	stopCh := make(chan struct{})
	defer wg.Wait()
	defer cancel()
	defer close(stopCh)

	ci := NewContainerInformer(cli)
	ci.SetResyncPeriod(time.Second)
	ci.Start(stopCh)

	namePrefix := RandomName("container")

	type result struct {
		created []string
		updated []string
		deleted []string
	}

	scenarios := []struct {
		name   string
		create []string
		update []string
		delete []string
	}{
		{
			name:   "add",
			create: []string{namePrefix + "1", namePrefix + "2", namePrefix + "3"},
		},
		{
			name:   "add-remove",
			create: []string{namePrefix + "4"},
			delete: []string{namePrefix + "1"},
		},
		{
			name:   "update-remove",
			update: []string{namePrefix + "2"},
			delete: []string{namePrefix + "3"},
		},
		{
			name:   "add-update-remove",
			create: []string{namePrefix + "1", namePrefix + "3"},
			update: []string{namePrefix + "2", namePrefix + "4"},
			delete: []string{namePrefix + "4"},
		},
		{
			name:   "remove",
			delete: []string{namePrefix + "1", namePrefix + "2", namePrefix + "3"},
		},
	}

	// Pulling image
	image := images.GetServiceControllerImageName()
	assert.Assert(t, cli.ImagePull(ctx, image))

	var containersLeft = map[string]bool{}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			var res result
			ci.AddInformer(&container.InformerBase[*container.Container]{
				Add: func(obj *container.Container) {
					if strings.HasPrefix(obj.Name, namePrefix) {
						res.created = append(res.created, obj.Name)
					}
				},
				Update: func(oldObj, newObj *container.Container) {
					if strings.HasPrefix(newObj.Name, namePrefix) {
						res.updated = append(res.updated, newObj.Name)
					}
				},
				Delete: func(obj *container.Container) {
					if strings.HasPrefix(obj.Name, namePrefix) {
						res.deleted = append(res.deleted, obj.Name)
					}
				},
			})

			// Creating containers
			for _, c := range s.create {
				assert.Assert(t, cli.ContainerCreate(&container.Container{
					Name:    c,
					Image:   image,
					Command: []string{"tail", "-f", "/dev/null"},
				}))
				containersLeft[c] = true
				assert.Assert(t, cli.ContainerStart(c))
			}

			// Restarting containers (update)
			for _, u := range s.update {
				assert.Assert(t, cli.ContainerRestart(u))
			}

			// Deleting containers
			for _, d := range s.delete {
				assert.Assert(t, cli.ContainerRemove(d))
				delete(containersLeft, d)
			}

			// Waiting 30 secs for state to match
			assert.Assert(t, utils.Retry(time.Second, 30, func() (bool, error) {
				match := reflect.DeepEqual(s.create, res.created) &&
					reflect.DeepEqual(s.update, res.updated) &&
					reflect.DeepEqual(s.delete, res.deleted)
				return match, nil
			}), "Created { expected: %v, got: %v } - Updated { expected: %v, got: %v } - Deleted { expected: %v, got: %v }",
				s.create, res.created, s.update, res.updated, s.delete, res.deleted)
		})
	}

	for c, _ := range containersLeft {
		t.Logf("removing container left: %s", c)
		_ = cli.ContainerRemove(c)
	}
}
