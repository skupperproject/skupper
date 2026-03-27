package compat

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper-libpod/v4/client/volumes_compat"
	"gotest.tools/v3/assert"
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

func TestGetDefaultContainerEndpoint(t *testing.T) {
	tests := []struct {
		name              string
		containerEndpoint string
		expected          string
	}{
		{
			name:              "uses CONTAINER_ENDPOINT when set",
			containerEndpoint: "unix:///tmp/docker.sock",
			expected:          "unix:///tmp/docker.sock",
		},
		{
			name:              "default endpoint",
			containerEndpoint: "",
			expected:          "unix:///run/user/1000/podman/podman.sock",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("CONTAINER_ENDPOINT", test.containerEndpoint)

			result := GetDefaultContainerEndpoint()
			assert.Equal(t, result, test.expected)

		})
	}
}
