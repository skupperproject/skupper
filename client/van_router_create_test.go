package client_test

import (
	"testing"

	"github.com/skupperproject/skupper/client"
	"github.com/stretchr/testify/assert"
)

func TestMustCreatOpenshiftRoutes(t *testing.T) {
	assert.False(t, client.MustCreateOpenshiftRoutes(true, true))
	assert.False(t, client.MustCreateOpenshiftRoutes(true, false))
	assert.False(t, client.MustCreateOpenshiftRoutes(false, true))
	assert.True(t, client.MustCreateOpenshiftRoutes(false, false))
}
