// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package revision

import (
	"context"
	"flag"
	"fmt"
	"log"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"namespacelabs.dev/foundation/std/go/server"
	ctrlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	runtimescheme "sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// Kubernetes controller settings.
	controllerPort = flag.Int("revision_controller_port", 10443, "Port that the Kubernetes controller binds to.")

	metricsAddress = flag.String("revision_controller_metrics_address", "0.0.0.0:19080",
		"Address port pair that the Kubernetes controller metrics endpoint binds to.")

	probeAddress = flag.String("revision_controller_health_probe_bind_address", "0.0.0.0:19081",
		"Address port pair that the Kubernetes controller health probe endpoint binds to.")
)

func WireService(context.Context, server.Registrar, ServiceDeps) {
	if err := setupControllers(); err != nil {
		log.Fatal(err)
	}
}

func setupControllers() error {
	groupVersion := schema.GroupVersion{Group: "k8s.namespacelabs.dev", Version: "v1alpha1"}
	schemeBuilder := &runtimescheme.Builder{GroupVersion: groupVersion}
	schemeBuilder.Register(&Revision{}, &RevisionList{})

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))

	zapopts := zap.Options{}
	ctrlruntime.SetLogger(zap.New(zap.UseFlagOptions(&zapopts)))

	mgr, err := ctrlruntime.NewManager(ctrlruntime.GetConfigOrDie(), ctrlruntime.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     *metricsAddress,
		Port:                   *controllerPort,
		HealthProbeBindAddress: *probeAddress,
		// No leader election until we figure out our HA requirements and design.
	})
	if err != nil {
		return fmt.Errorf("failed to start the controller manager: %+v", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up healthz: %+v", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up readyz: %+v", err)
	}

	if err := ctrlruntime.NewControllerManagedBy(mgr).
		For(&Revision{}).
		Complete(&RevisionReconciler{
			clt:      mgr.GetClient(),
			recorder: mgr.GetEventRecorderFor("revision"),
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
