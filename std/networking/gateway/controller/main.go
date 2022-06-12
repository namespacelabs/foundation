// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"flag"
	"log"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	controllerscheme "sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// HTTP listening address:port pair.
	httpEnvoyListenAddr = flag.String("http_envoy_listen_address", "0.0.0.0:10000", "HTTP address that Envoy should listen on.")

	debug = flag.Bool("debug", true, "Enable xDS gRPC server debug logging, giving us visibility into each snapshot update.")

	// The address:port pair that the xDS server listens on.
	xdsServerAddress = flag.String("xds_server_port", "127.0.0.1:18000", "xDS gRPC management server port.")

	xdsClusterName = flag.String("xds_cluster_name", "xds_cluster", "xDS cluster name.")

	// The address:port pair that the ALS server listens on.
	alsServerAddress = flag.String("als_server_address", "127.0.0.1:18090", "ALS gRPC server port.")

	alsClusterName = flag.String("als_cluster_name", "als_cluster", "ALS cluster name.")

	// Tell Envoy to use this Node ID.
	nodeID = flag.String("node_id", "envoy_node", "Envoy Node ID used for cache snapshots.")

	// Kubernetes controller settings.
	controllerPort = flag.Int("controller_port", 18443, "Port that the Kubernetes controller binds to.")

	metricsAddress = flag.String("controller_metrics_address", ":18080",
		"Address that the Kubernetes controller metrics endpoint binds to.")

	probeAddress = flag.String("controller_health_probe_bind_address", ":18081",
		"Address that the Kubernetes controller health probe endpoint binds to.")

	enableLeaderElection = flag.Bool("controller_enable_leader_election", false,
		"Enable leader election for the Kubernetes controller manager, with true guaranteeing only one active controller manager.")
)

func main() {
	flag.Parse()

	xdsAddrPort, err := ParseAddressPort(*xdsServerAddress)
	if err != nil {
		log.Fatalf("failed to parse xDS server address: %v", err)
	}

	alsAddrPort, err := ParseAddressPort(*alsServerAddress)
	if err != nil {
		log.Fatalf("failed to parse ALS server address: %v", err)
	}

	logger := Logger{}
	logger.Debug = *debug

	transcoderSnapshot := NewTranscoderSnapshot(
		WithEnvoyNodeId(*nodeID),
		WithLogger(logger),
		WithXdsCluster(*xdsClusterName, xdsAddrPort),
		WithAlsCluster(*alsClusterName, alsAddrPort),
	)

	if err := transcoderSnapshot.RegisterHttpListener(*httpEnvoyListenAddr); err != nil {
		log.Fatal(err)
	}
	log.Printf("registered HTTP listener on %s\n", *httpEnvoyListenAddr)

	// SetupSignalHandler registers for SIGTERM and SIGINT. A context is returned
	// which is canceled on one of these signals. If a second signal is caught, the program
	// is terminated with exit code 1.
	ctx := ctrl.SetupSignalHandler()

	xdsServer := NewXdsServer(ctx, transcoderSnapshot.cache, logger)
	xdsServer.RegisterServices()
	log.Println("registered xDS services")

	// Run the Kubernetes controller responsible for handling the `HttpGrpcTranscoder` custom resource.

	// Every set of controllers needs a `Scheme` (https://book.kubebuilder.io/cronjob-tutorial/gvks.html#err-but-whats-that-scheme-thing),
	// which provides mappings between Kinds and their corresponding Go types.
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	groupVersion := schema.GroupVersion{Group: "k8s.namespacelabs.dev", Version: "v1"}
	schemeBuilder := &controllerscheme.Builder{GroupVersion: groupVersion}
	schemeBuilder.Register(&HttpGrpcTranscoder{}, &HttpGrpcTranscoderList{})
	if err := schemeBuilder.AddToScheme(scheme); err != nil {
		log.Fatalf("failed to add the HttpGrpcTranscoder scheme: %+v", err)
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
		log.Fatalf("failed to start the controller manager: %+v", err)
	}

	// Add healthz.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatalf("failed to set up healthz: %+v", err)
	}
	log.Println("set up healthz for the controller manager")

	// Add readyz.
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Fatalf("failed to set up readyz: %+v", err)
	}
	log.Println("set up readyz for the controller manager")

	httpGrpcTranscoderReconciler := HttpGrpcTranscoderReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		snapshot: transcoderSnapshot,
	}
	if err := httpGrpcTranscoderReconciler.SetupWithManager(mgr); err != nil {
		log.Fatalf("failed to set up the HTTP gRPC Transcoder reconciler: %+v", err)
	}
	log.Println("set up the HTTP gRPC Transcoder reconciler")

	errChan := make(chan error)
	go func() {
		log.Printf("starting xDS server on port %d\n", xdsAddrPort.port)
		errChan <- xdsServer.Start(ctx, xdsAddrPort.port)
	}()

	go func() {
		log.Printf("starting the controller manager on port %d\n", *controllerPort)
		errChan <- mgr.Start(ctx)
	}()

	select {
	case err := <-errChan:
		log.Fatalf("killing the controller manager: %v", err)
	case <-ctx.Done():
		log.Fatalf("killing the controller manager: %v", ctx.Err())
	}
}
