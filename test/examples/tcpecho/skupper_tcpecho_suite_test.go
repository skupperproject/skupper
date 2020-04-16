package tcpecho_test

import (
	"github.com/rh-messaging/qdr-shipshape/pkg/testcommon"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSkupper(t *testing.T) {
	RegisterFailHandler(Fail)
	testcommon.RunSpecs(t, "tcpecho", "Skupper TCP Echo Example Test Suite")
}

func TestMain(m *testing.M) {
	testcommon.Initialize(m)
}
