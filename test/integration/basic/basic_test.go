// +build integration

package basic

import (
	"context"
	"testing"
)

func TestBasic(t *testing.T) {
	testRunner := &BasicTestRunner{}

	testRunner.Build(t, "basic")
	testRunner.Run(context.Background())
}
