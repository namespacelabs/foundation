// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"flag"
	"os"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	controllerscheme "sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	debug = flag.Bool("debug", false, "Enable xDS server debug logging")

	// The port that this xDS server listens on
	xdsPort = flag.Uint("xds_server_port", 18000, "xDS management server port")

	// Tell Envoy to use this Node ID
	nodeID = flag.String("node_id", "envoy_node", "Node ID")

	controllerPort = flag.Int("controller_port", 18443, "Port that the Kubernetes controller binds to")

	metricsAddress = flag.String("controller_metrics_address", ":18080",
		"Address that the Kubernetes controller metrics endpoint binds to")

	probeAddress = flag.String("controller_health_probe_bind_address", ":18081",
		"Address that the Kubernetes controller health probe endpoint binds to")

	enableLeaderElection = flag.Bool("controller_enable_leader_election", false,
		"Enable leader election for the Kubernetes controller manager, with true guaranteeing only one active controller manager.")
)

func main() {
	flag.Parse()

	l := Logger{}
	l.Debug = *debug

	// Create a cache
	cache := cache.NewSnapshotCache(false, cache.IDHash{}, l)

	// Create the snapshot that we'll serve to Envoy.
	snapshot := GenerateSnapshot()
	if err := snapshot.Consistent(); err != nil {
		l.Errorf("snapshot inconsistency: %+v\n%+v", snapshot, err)
		os.Exit(1)
	}
	l.Debugf("will serve snapshot %+v", snapshot)

	// Add the snapshot to the cache.
	if err := cache.SetSnapshot(context.Background(), *nodeID, snapshot); err != nil {
		l.Errorf("snapshot error %+v for %+v", err, snapshot)
		os.Exit(1)
	}

	// Run the xDS server.
	ctx := context.Background()
	cb := &test.Callbacks{Debug: l.Debug}
	srv := server.NewServer(ctx, cache, cb)
	if err := RunXdsServer(ctx, srv, *xdsPort); err != nil {
		l.Errorf("failed to start the xDS server on port %d: %+v", *xdsPort, err)
		os.Exit(1)
	}

	// Run the Kubernetes controller responsible for handling the `HttpGrpcTranscoder` custom resource.

	// Every set of controllers needs a `Scheme` (https://book.kubebuilder.io/cronjob-tutorial/gvks.html#err-but-whats-that-scheme-thing),
	// which provides mappings between Kinds and their corresponding Go types.
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	groupVersion := schema.GroupVersion{Group: "k8s.namespacelabs.dev", Version: "v1"}
	schemeBuilder := &controllerscheme.Builder{GroupVersion: groupVersion}
	schemeBuilder.Register(&HttpGrpcTranscoder{}, &HttpGrpcTranscoderList{})
	if err := schemeBuilder.AddToScheme(scheme); err != nil {
		l.Errorf("failed to add the HttpGrpcTranscoder scheme: %+v", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     *metricsAddress,
		Port:                   *controllerPort,
		HealthProbeBindAddress: *probeAddress,
		LeaderElection:         *enableLeaderElection,
		// We follow the schematic from https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/emptymain.go#L151
		// and other canonical examples.
		LeaderElectionID: "63245986.k8s.namespacelabs.dev",
	})
	if err != nil {
		l.Errorf("failed to start the controller manager: %+v", err)
		os.Exit(1)
	}

	// Add healthz.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		l.Errorf("failed to set up healthz: %+v", err)
		os.Exit(1)
	}
	// Add readyz.
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		l.Errorf("failed to set up readyz: %+v", err)
		os.Exit(1)
	}

	httpGrpcTranscoderReconciler := HttpGrpcTranscoderReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		cache:  cache,
	}
	if err := httpGrpcTranscoderReconciler.SetupWithManager(mgr); err != nil {
		l.Errorf("failed to set up the HTTP gRPC Transcoder reconciler: %+v", err)
		os.Exit(1)
	}

	l.Infof("starting the controller manager on port %d", *controllerPort)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		l.Errorf("failed to start the controller manager: %+v", err)
		os.Exit(1)
	}
}
