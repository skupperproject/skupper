//go:build integration

package kubecontrollertest

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/fixtures"
	"github.com/skupperproject/skupper/internal/utils"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func waitFor(t *testing.T, timeout, interval time.Duration, fn func() (bool, error)) {
	t.Helper()
	err := utils.Retry(interval, int(timeout/interval), fn)
	assert.NilError(t, err)
}

func retryOnNotFound(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

func listenerWithHostPort(name, namespace, host string, port int) *skupperv2alpha1.Listener {
	l := fixtures.Listener(name, namespace)
	l.Spec.Host = host
	l.Spec.Port = port
	return l
}

func routerSelector() map[string]string {
	return map[string]string{
		"skupper.io/component": "router",
		"application":          "skupper-router",
	}
}

func verifyStatus(t *testing.T, expected, actual skupperv2alpha1.Status) {
	t.Helper()
	assert.Equal(t, expected.StatusType, actual.StatusType, actual.Message)
	assert.Equal(t, expected.Message, actual.Message)
	for _, c := range expected.Conditions {
		existing := meta.FindStatusCondition(actual.Conditions, c.Type)
		assert.Assert(t, existing != nil)
		assert.Equal(t, c.Status, existing.Status)
		assert.Equal(t, c.Reason, existing.Reason)
		if c.Message != "" {
			assert.Equal(t, c.Message, existing.Message)
		}
	}
}
