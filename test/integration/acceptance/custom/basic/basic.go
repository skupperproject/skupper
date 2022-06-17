package basic

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/env"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SkipReasonIngressNone           = "this test only runs against a single cluster (ingress=none)"
	SkipReasonNodePortNoIngressHost = "this test can only be executed if PUBLIC_1_INGRESS_HOST environment varialbe is set (ingress=nodeport)"
)

type BasicTestRunner struct {
	base.ClusterTestRunnerBase
}

func (r *BasicTestRunner) RunTests(ctx context.Context, t *testing.T) {

	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, pubCluster, 1))
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, prvCluster, 1))
}

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.SiteConfigSpec, createOptsPrivate types.SiteConfigSpec, tokenType string, testSync bool, t *testing.T) {
	var err error
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	createOptsPublic.SkupperNamespace = pub1Cluster.Namespace
	siteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPublic)
	assert.Assert(t, err)
	err = pub1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err)

	const secretFile = "/tmp/public_basic_1_secret.yaml"
	if tokenType == "claim" {
		err = pub1Cluster.VanClient.TokenClaimCreateFile(ctx, types.DefaultVanName, []byte(createOptsPublic.Password), 15*time.Minute, 1, secretFile)
	} else {
		err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	}
	assert.Assert(t, err)

	createOptsPrivate.SkupperNamespace = prv1Cluster.Namespace
	siteConfig, err = prv1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPrivate)
	assert.Assert(t, err)
	err = prv1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err)

	var podStartTimeBefore *v1.Time
	if testSync == true {
		// Pick the pod details for config-sync validation
		podsRouter, _ := kube.GetPods("skupper.io/component=router", prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient)
		assert.Assert(t, len(podsRouter) > 0)
		podStartTimeBefore = podsRouter[0].Status.StartTime
	}

	var connectorCreateOpts = types.ConnectorCreateOptions{
		SkupperNamespace: prv1Cluster.Namespace,
		Name:             "",
		Cost:             0,
	}
	_, err = prv1Cluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	assert.Assert(t, err)

	var podStartTimeAfter *v1.Time
	if testSync == true {
		assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, prv1Cluster, 1))

		linkStatus, err := prv1Cluster.VanClient.ConnectorInspect(context.Background(), "link1")
		assert.Assert(t, err)
		if err != nil {
			log.Fatalf("[TestSync] error: %v", err)
			return
		}
		assert.Assert(t, linkStatus.Connected == true)

		podsRouter, _ := kube.GetPods("skupper.io/component=router", prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient)
		assert.Assert(t, len(podsRouter) > 0)
		podStartTimeAfter = podsRouter[0].Status.StartTime

		// Check if the container has restarted
		assert.Assert(t, podStartTimeAfter.Equal(podStartTimeBefore))

		// Check if the Volume is shared by both containers
		podContainers, _ := kube.GetReadyPod(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, "router")
		for _, container := range podContainers.Spec.Containers {
			foundCertVol := false
			for _, contVolume := range container.VolumeMounts {
				if contVolume.Name == "skupper-router-certs" {
					foundCertVol = true
				}
			}
			assert.Assert(t, foundCertVol == true)
		}
	}
}

func (r *BasicTestRunner) Delete(ctx context.Context, t *testing.T) {
	ctx, cn := context.WithTimeout(ctx, constants.NamespaceDeleteTimeout)
	defer cn()
	pub1Cluster, _ := r.GetPublicContext(1)
	prv1Cluster, _ := r.GetPrivateContext(1)
	if err := pub1Cluster.VanClient.SiteConfigRemove(ctx); err != nil {
		t.Logf("error removing site config: %v", err)
		t.Logf("removing router - err: %v", pub1Cluster.VanClient.RouterRemove(ctx))
	}
	if err := prv1Cluster.VanClient.SiteConfigRemove(ctx); err != nil {
		t.Logf("error removing site config: %v", err)
		t.Logf("removing router - err: %v", prv1Cluster.VanClient.RouterRemove(ctx))
	}
	waitNoPods := func(componentSelector string, cluster *base.ClusterContext) error {
		return utils.RetryWithContext(ctx, time.Second, func() (bool, error) {
			pods, _ := kube.GetPods(componentSelector, cluster.Namespace, cluster.VanClient.KubeClient)
			return len(pods) == 0, nil
		})
	}
	assert.Assert(t, waitNoPods("skupper.io/component=service-controller", pub1Cluster))
	assert.Assert(t, waitNoPods("skupper.io/component=router", pub1Cluster))
	assert.Assert(t, waitNoPods("skupper.io/component=service-controller", prv1Cluster))
	assert.Assert(t, waitNoPods("skupper.io/component=router", prv1Cluster))
}

func (r *BasicTestRunner) TearDown(ctx context.Context) {
	errMsg := "Something failed! aborting teardown"

	pub, err := r.GetPublicContext(1)
	if err != nil {
		log.Warn(errMsg)
	}

	priv, err := r.GetPrivateContext(1)
	if err != nil {
		log.Warn(errMsg)
	}

	_ = pub.DeleteNamespace()
	_ = priv.DeleteNamespace()
}

func (r *BasicTestRunner) Run(ctx context.Context, t *testing.T) {
	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	testcases := []struct {
		id                string
		doc               string
		skip              bool
		skipReason        string
		tokenType         string
		testSync          bool
		createOptsPublic  types.SiteConfigSpec
		createOptsPrivate types.SiteConfigSpec
	}{
		{
			id:         "test-sync-container",
			doc:        "Test the config-sync container",
			skip:       base.MultipleClusters(),
			skipReason: SkipReasonIngressNone,
			testSync:   true,
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
		},
		{
			id:         "interiors-ingress-none",
			doc:        "Connecting two interiors with ingress=none",
			skip:       base.MultipleClusters(),
			skipReason: SkipReasonIngressNone,
			testSync:   false,
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
		},
		{
			id:       "interiors-ingress-default",
			doc:      "Connecting two interiors with ingress=default (route if available or loadbalancer)",
			testSync: false,
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressDefault(),
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressDefault(),
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
		},
		{
			id:         "edge-interior-ingress-none",
			doc:        "Connecting a private edge to a public interior with ingress=none",
			skip:       base.MultipleClusters(),
			skipReason: SkipReasonIngressNone,
			testSync:   false,
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeEdge),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
		},
		{
			id:         "interiors-ingress-nodeport",
			doc:        "Connecting two interiors with ingress=nodeport",
			skip:       os.Getenv(env.Public1IngressHost) == "",
			skipReason: SkipReasonNodePortNoIngressHost,
			tokenType:  "claim",
			testSync:   false,
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     true,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNodePortString,
				Router: constants.DefaultRouterOptions(&types.RouterOptions{
					IngressHost: os.Getenv(env.Public1IngressHost),
				}),
				Controller: types.ControllerOptions{
					IngressHost: os.Getenv(env.Public1IngressHost),
				},
				Replicas: 1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressDefault(),
				Replicas:          1,
				Router:            constants.DefaultRouterOptions(nil),
			},
		},
	}

	defer r.TearDown(ctx)
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)
	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)
	err = pub1Cluster.CreateNamespace()
	assert.Assert(t, err)
	err = prv1Cluster.CreateNamespace()
	assert.Assert(t, err)

	for _, c := range testcases {
		t.Run(c.id, func(t *testing.T) {
			if c.skip {
				t.Skipf("Skipping: %s [%s]\n", c.doc, c.skipReason)
			}
			t.Logf("Testing: %s\n", c.doc)
			defer r.Delete(ctx, t)
			r.Setup(ctx, c.createOptsPublic, c.createOptsPrivate, c.tokenType, c.testSync, t)
			r.RunTests(ctx, t)
		})
	}
}
