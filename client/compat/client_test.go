package compat

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes_compat"
	"gotest.tools/assert"
)

func TestToAPIError(t *testing.T) {
	notFoundErr := volumes_compat.NewVolumeDeleteNotFound()
	assert.Assert(t, notFoundErr != nil)
	notFoundErr.Payload = new(volumes_compat.VolumeDeleteNotFoundBody)
	notFoundErr.Payload.Message = "Sample error message"
	notFoundErr.Payload.Because = "Because it has to fail"
	notFoundErr.Payload.ResponseCode = 404
	// validating result only and both result and error
	for _, test := range []struct {
		actualError          error
		expectedMessage      string
		expectedBecause      string
		expectedResponseCode int64
	}{
		{
			actualError:          notFoundErr,
			expectedMessage:      notFoundErr.Payload.Message,
			expectedBecause:      notFoundErr.Payload.Because,
			expectedResponseCode: notFoundErr.Payload.ResponseCode,
		}, {
			actualError:     fmt.Errorf("unused error"),
			expectedMessage: "unused error",
		},
	} {
		apiErr := ToAPIError(test.actualError)
		assert.Assert(t, apiErr != nil)
		assert.Equal(t, apiErr.Message, test.expectedMessage)
		assert.Equal(t, apiErr.Because, test.expectedBecause)
		assert.Equal(t, apiErr.ResponseCode, test.expectedResponseCode)
	}

	// validating none
	assert.Assert(t, ToAPIError(nil) == nil)
}
