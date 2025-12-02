package controller

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestInputResourceHandler(t *testing.T) {

	t.Run("handler created for docker platform", func(t *testing.T) {
		t.Setenv("CONTAINER_ENGINE", "docker")
		inputResourceHandler := NewInputResourceHandler("test_namespace", "test_inputPath", mockBootstrap, mockPostExec)
		expectedConfigBootstrap := bootstrap.Config{
			Namespace: "test_namespace",
			InputPath: "test_inputPath",
			Platform:  "docker",
			Binary:    "docker",
		}

		assert.Assert(t, inputResourceHandler != nil)
		assert.Assert(t, inputResourceHandler.inputPath == "test_inputPath")
		assert.Assert(t, inputResourceHandler.ConfigBootstrap == expectedConfigBootstrap)
	})

	t.Run("handler created for podman platform", func(t *testing.T) {
		t.Setenv("CONTAINER_ENGINE", "podman")
		inputResourceHandler := NewInputResourceHandler("test_namespace", "test_inputPath", mockBootstrap, mockPostExec)
		expectedConfigBootstrap := bootstrap.Config{
			Namespace: "test_namespace",
			InputPath: "test_inputPath",
			Platform:  "podman",
			Binary:    "podman",
		}

		assert.Assert(t, inputResourceHandler != nil)
		assert.Assert(t, inputResourceHandler.inputPath == "test_inputPath")
		assert.Assert(t, inputResourceHandler.ConfigBootstrap == expectedConfigBootstrap)
	})

	t.Run("handler not created for linux platform", func(t *testing.T) {
		t.Setenv("CONTAINER_ENGINE", "linux")
		inputResourceHandler := NewInputResourceHandler("test_namespace", "test_inputPath", mockBootstrap, mockPostExec)

		assert.Assert(t, inputResourceHandler == nil)

	})

	t.Run("handler not created because the system reload is configured to be manual", func(t *testing.T) {
		t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, "manual")
		inputResourceHandler := NewInputResourceHandler("test_namespace", "test_inputPath", mockBootstrap, mockPostExec)

		assert.Assert(t, inputResourceHandler == nil)

	})

	t.Run("handler not created for unknown platform", func(t *testing.T) {
		t.Setenv("CONTAINER_ENGINE", "unknown")
		inputResourceHandler := NewInputResourceHandler("test_namespace", "test_inputPath", mockBootstrap, mockPostExec)

		assert.Assert(t, inputResourceHandler == nil)

	})

	t.Run("resource file created or updated", func(t *testing.T) {
		namespace := "test-file-created-ns"
		inputPath := "test-file-created-input-path"

		handler := NewInputResourceHandler(namespace, inputPath, mockBootstrap, mockPostExec)

		logSpy := &testLogHandler{
			handler: slog.Default().Handler(),
		}
		handler.logger = slog.New(logSpy)

		resourceName := "site.yaml"
		handler.OnCreate(resourceName)

		expectedMsg := fmt.Sprintf("Resource has been created: %s", resourceName)
		if count := logSpy.Count(expectedMsg); count != 1 {
			t.Errorf("Expected log '%s' to be present, but found count: %d", expectedMsg, count)
		}

	})

	t.Run("resource file removed", func(t *testing.T) {
		namespace := "test-file-ns"
		inputPath := "test-file-input-path"

		handler := NewInputResourceHandler(namespace, inputPath, mockBootstrap, mockPostExec)

		logSpy := &testLogHandler{
			handler: slog.Default().Handler(),
		}
		handler.logger = slog.New(logSpy)

		resourceName := "site.yaml"
		handler.OnRemove(resourceName)

		expectedMsg := fmt.Sprintf("Resource has been deleted: %s", resourceName)
		if count := logSpy.Count(expectedMsg); count != 1 {
			t.Errorf("Expected log '%s' to be present, but found count: %d", expectedMsg, count)
		}
	})

	t.Run("resource file created or updated but the reload fails", func(t *testing.T) {
		namespace := "test-file-created-ns"
		inputPath := "test-file-created-input-path"

		handler := NewInputResourceHandler(namespace, inputPath, mockBootstrapFailed, mockPostExec)

		logSpy := &testLogHandler{
			handler: slog.Default().Handler(),
		}
		handler.logger = slog.New(logSpy)

		resourceName := "site.yaml"
		handler.OnCreate(resourceName)

		expectedMsg := fmt.Sprintf("Failed to bootstrap: failed to bootstrap")
		if count := logSpy.Count(expectedMsg); count != 1 {
			t.Errorf("Expected log '%s' to be present, but found count: %d", expectedMsg, count)
		}

	})

}

func mockBootstrap(config *bootstrap.Config) (*api.SiteState, error) {
	return api.NewSiteState(false), nil
}
func mockPostExec(config *bootstrap.Config, siteState *api.SiteState) {
	fmt.Println("post bootstrap execution completed")
}

func mockBootstrapFailed(config *bootstrap.Config) (*api.SiteState, error) {
	return nil, fmt.Errorf("failed to bootstrap")
}
