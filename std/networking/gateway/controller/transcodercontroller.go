// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HttpGrpcTranscoderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	cache  cache.SnapshotCache
}

func (r *HttpGrpcTranscoderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *HttpGrpcTranscoderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&HttpGrpcTranscoder{}).
		Complete(r)
}
