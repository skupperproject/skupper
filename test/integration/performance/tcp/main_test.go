package tcp

import (
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}
