package controller

import (
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/v3/assert"
)

func TestNewSystemAdaptorHandler_ManualReturnsNil(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeManual)
	h := NewSystemAdaptorHandler("ns")
	assert.Assert(t, h == nil)
}

func TestNewSystemAdaptorHandler_AutoReturnsHandler(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	h := NewSystemAdaptorHandler("ns")
	assert.Assert(t, h != nil)
	assert.Equal(t, h.namespace, "ns")
	assert.Assert(t, h.logger != nil)
}

func TestNewSystemAdaptorHandler_ErrorGettingLocalRouterAddress(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	handler := NewSystemAdaptorHandler("ns")
	assert.Assert(t, handler != nil)
	handler.Start(nil)
	assert.Assert(t, handler.systemAdaptor == nil)

}

func TestNewSystemAdaptorHandler_Stop(t *testing.T) {
	t.Setenv(types.ENV_SYSTEM_AUTO_RELOAD, types.SystemReloadTypeAuto)
	handler := NewSystemAdaptorHandler("ns")
	assert.Assert(t, handler != nil)
	handler.running = true
	handler.Stop()
	assert.Assert(t, handler.running == false)

}
