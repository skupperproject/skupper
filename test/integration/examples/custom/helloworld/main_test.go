package helloworld

import (
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
)

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}
