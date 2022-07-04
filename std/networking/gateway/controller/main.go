// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"flag"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	controllerscheme "sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// HTTP listening address:port pair.
	httpEnvoyListenAddress = flag.String("http_envoy_listen_address", "0.0.0.0:10000", "HTTP address that Envoy should listen on.")

	debug = flag.Bool("debug", false, "Enable xDS gRPC server debug logging, giving us visibility into each snapshot update. "+
		"We additionally enable development logging for the Kubernetes controller.")

	// The address:port pair that the xDS server listens on.
	xdsServerAddress = flag.String("xds_server_port", "127.0.0.1:18000", "xDS gRPC management address:port pair.")

	xdsClusterName = flag.String("xds_cluster_name", "xds_cluster", "xDS cluster name.")

	// The address:port pair that the ALS server listens on.
	alsServerAddress = flag.String("als_server_address", "127.0.0.1:18090", "ALS gRPC server address:port pair.")

	alsClusterName = flag.String("als_cluster_name", "als_cluster", "ALS cluster name.")

	// Tell Envoy to use this Node ID.
	nodeID = flag.String("node_id", "envoy_node", "Envoy Node ID used for cache snapshots.")

	// Kubernetes controller settings.
	controllerPort = flag.Int("controller_port", 18443, "Port that the Kubernetes controller binds to.")

	metricsAddress = flag.String("controller_metrics_address", ":18080",
		"Address port pair that the Kubernetes controller metrics endpoint binds to.")

	probeAddress = flag.String("controller_health_probe_bind_address", ":18081",
		"Address port pair that the Kubernetes controller health probe endpoint binds to.")

	enableLeaderElection = flag.Bool("controller_enable_leader_election", false,
		"Enable leader election for the Kubernetes controller manager, with true guaranteeing only one active controller manager.")
)

func main() {
	flag.Parse()

	controllerNamespace := os.Getenv("FN_KUBERNETES_NAMESPACE")
	if controllerNamespace == "" {
		log.Fatal("FN_KUBERNETES_NAMESPACE is required")
	}

	log.Printf("Observing namespace %q...", controllerNamespace)

	xdsAddrPort, err := ParseAddressPort(*xdsServerAddress)
	if err != nil {
		log.Fatalf("failed to parse xDS server address: %v", err)
	}

	alsAddrPort, err := ParseAddressPort(*alsServerAddress)
	if err != nil {
		log.Fatalf("failed to parse ALS server address: %v", err)
	}

	logger := NewLogger(*debug)

	transcoderSnapshot := NewTranscoderSnapshot(
		WithEnvoyNodeId(*nodeID),
		WithLogger(logger),
		WithXdsCluster(*xdsClusterName, xdsAddrPort),
		WithAlsCluster(*alsClusterName, alsAddrPort),
	)

	if err := transcoderSnapshot.RegisterHttpListener(*httpEnvoyListenAddress); err != nil {
		log.Fatal(err)
	}
	logger.Infof("registered HTTP listener on %s\n", *httpEnvoyListenAddress)

	// SetupSignalHandler registers for SIGTERM and SIGINT. A context is returned
	// which is canceled on one of these signals. If a second signal is caught, the program
	// is terminated with exit code 1.
	ctx := ctrl.SetupSignalHandler()

	xdsServer := NewXdsServer(ctx, transcoderSnapshot.cache, logger)
	xdsServer.RegisterServices()

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

	ctrl.SetLogger(zap.New(zap.UseDevMode(*debug)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     *metricsAddress,
		Port:                   *controllerPort,
		HealthProbeBindAddress: *probeAddress,
		LeaderElection:         *enableLeaderElection,
		// We follow the schematic from https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/emptymain.go#L151
		// and other canonical examples.
		LeaderElectionID: "63245986.k8s.namespacelabs.dev",
		Namespace:        controllerNamespace,
	})
	if err != nil {
		log.Fatalf("failed to start the controller manager: %+v", err)
	}

	// Add healthz.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatalf("failed to set up healthz: %+v", err)
	}

	// Add readyz.
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Fatalf("failed to set up readyz: %+v", err)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: "http-grpc-transcoder-controller"})

	httpGrpcTranscoderReconciler := HttpGrpcTranscoderReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		snapshot: transcoderSnapshot,
		recorder: recorder,
	}
	if err := httpGrpcTranscoderReconciler.SetupWithManager(mgr, controllerNamespace); err != nil {
		log.Fatalf("failed to set up the HTTP gRPC Transcoder reconciler: %+v", err)
	}

	errChan := make(chan error)
	go func() {
		logger.Infof("starting xDS server on port %d\n", xdsAddrPort.port)
		errChan <- xdsServer.Start(ctx, xdsAddrPort.port)
	}()

	go func() {
		logger.Infof("starting the controller manager on port %d\n", *controllerPort)
		errChan <- mgr.Start(ctx)
	}()

	select {
	case err := <-errChan:
		log.Fatalf("killing the controller manager: %v", err)
	case <-ctx.Done():
		log.Fatalf("killing the controller manager: %v", ctx.Err())
	}
}
