package tcpecho

import (
	"github.com/onsi/ginkgo"
	"github.com/rh-messaging/qdr-shipshape/pkg/testcommon"
	"github.com/rh-messaging/shipshape/pkg/framework"
	"github.com/rh-messaging/shipshape/pkg/framework/operators"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	Config                        *testcommon.Config
	FrameworkNSPublic             *framework.Framework
	FrameworkNSPrivate            *framework.Framework
	PubCtx                        *framework.ContextData
	PrvCtx                        *framework.ContextData
	TcpEchoDeploymentGroupVersion = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
)

const (
	ENV_CLUSTER_LOCAL = "CLUSTER_LOCAL"
)

var _ = ginkgo.BeforeEach(func() {

	// Loading configuration (ini or environment)
	Config = testcommon.LoadConfig("skupper/tcpecho")

	// Initializes using only Skupper Operator
	skupperOperator := operators.SupportedOperators[operators.OperatorTypeSkupper].(*operators.SkupperOperatorBuilder)

	clusterLocal := Config.GetEnvPropertyBool(ENV_CLUSTER_LOCAL, true)
	skupperOperator.ClusterLocal(clusterLocal)

	// This test is using a single context with two instances
	// it can be modified to use multiple contexts with one
	// framework instance as well
	publicBuilder := framework.
		NewFrameworkBuilder("skupper-tcpecho-public").
		WithBuilders(skupperOperator)

	privateBuilder := framework.
		NewFrameworkBuilder("skupper-tcpecho-private").
		WithBuilders(skupperOperator)

	// Deploying to both namespaces
	FrameworkNSPublic = publicBuilder.Build()
	FrameworkNSPrivate = privateBuilder.Build()

	// Kubernetes contexts for both public and private namespaces
	PubCtx = FrameworkNSPublic.GetFirstContext()
	PrvCtx = FrameworkNSPrivate.GetFirstContext()

})
