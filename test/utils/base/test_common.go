package base

import (
	"context"
	"log"
	"os"
	"testing"
)

type BasicTopologySetup struct {
	TestRunner       *ClusterTestRunnerBase
	NamespaceId      string
	PreSkupperSetup  func(testRunner *ClusterTestRunnerBase) error
	PostSkupperSetup func(testRunner *ClusterTestRunnerBase) error
}

func RunBasicTopologyTests(m *testing.M, topology BasicTopologySetup) {
	ParseFlags()

	// Local vars
	testRunner := topology.TestRunner
	namespaceId := topology.NamespaceId

	// internal tearDown function
	tearDownFn := func() {
		if len(topology.TestRunner.ClusterContexts) > 0 {
			TearDownSimplePublicAndPrivate(testRunner)
		}
	}
	// internal helper to exit without running the tests
	exit := func(format string, v ...interface{}) {
		log.Printf(format, v...)
		tearDownFn()
		os.Exit(1)
	}

	// Basic 2 namespaces setup only
	clusterNeeds := ClusterNeeds{
		NamespaceId:     namespaceId,
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	if err := testRunner.Validate(clusterNeeds); err != nil {
		log.Printf("gateway tests cannot be executed: %s", err)
		return
	}
	if _, err := testRunner.Build(clusterNeeds, nil); err != nil {
		log.Printf("error preparing cluster contexts: %s", err)
		return
	}

	// Setting up teardown
	defer tearDownFn()
	HandleInterruptSignal(tearDownFn)

	// Creating namespaces
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	if err := SetupSimplePublicPrivate(ctx, testRunner); err != nil {
		log.Printf("error setting up public and private namespaces: %s", err)
		os.Exit(1)
	}

	// If a pre Skupper setup hook provided, run it
	if topology.PreSkupperSetup != nil {
		if err := topology.PreSkupperSetup(testRunner); err != nil {
			log.Printf("error executing pre skupper setup hook: %s", err)
			os.Exit(1)
		}
	}

	// Connecting Skupper sites
	if err := ConnectSimplePublicPrivate(ctx, testRunner); err != nil {
		log.Printf("error connecting public and private namespaces: %s", err)
		os.Exit(1)
	}

	// Wait for sites to be connected
	pub, _ := testRunner.GetPublicContext(1)
	if err := WaitForSkupperConnectedSites(ctx, pub, 1); err != nil {
		exit("timed out waiting for skupper sites to be connected: %s", err)
	}

	// If a post Skupper setup hook provided, run it
	if topology.PostSkupperSetup != nil {
		if err := topology.PostSkupperSetup(testRunner); err != nil {
			log.Printf("error executing post skupper setup hook: %s", err)
			os.Exit(1)
		}
	}

	// Running package level tests
	m.Run()

}
