// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package controllers

import (
	"context"
	"flag"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var (
	// Kubernetes controller settings.
	controllerPort = flag.Int("controller_port", 18443, "Port that the Kubernetes controller binds to.")

	metricsAddress = flag.String("controller_metrics_address", "0.0.0.0:18080",
		"Address port pair that the Kubernetes controller metrics endpoint binds to.")

	probeAddress = flag.String("controller_health_probe_bind_address", "0.0.0.0:18081",
		"Address port pair that the Kubernetes controller health probe endpoint binds to.")

	enableLeaderElection = flag.Bool("controller_enable_leader_election", false,
		"Enable leader election for the Kubernetes controller manager, with true guaranteeing only one active controller manager.")
)

func Prepare(ctx context.Context, _ ExtensionDeps) error {
	mgr, err := controllerruntime.NewManager(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		MetricsBindAddress:     *metricsAddress,
		Port:                   *controllerPort,
		HealthProbeBindAddress: *probeAddress,
		LeaderElection:         *enableLeaderElection,
		LeaderElectionID:       "64367099.k8s.namespacelabs.dev",
	})

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up healthz: %+v", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up readyz: %+v", err)
	}

	if err != nil {
		return fmt.Errorf("failed to start the controller manager: %+v", err)
	}

	if err := controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}).
		Complete(RuntimeConfigReconciler{
			Client: mgr.GetClient(),
		}); err != nil {
		return err
	}

	go func() {
		// XXX we don't have a good way to model background work.
		// Do not use the received context, as that has a timeout built-in.
		if err := mgr.Start(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()

	return nil
}