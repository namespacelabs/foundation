// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HttpGrpcTranscoderReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	snapshot *TranscoderSnapshot
}

func (r *HttpGrpcTranscoderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	transcoder := &HttpGrpcTranscoder{}
	if err := r.Get(ctx, req.NamespacedName, transcoder); err != nil {
		if apierrors.IsNotFound(err) {
			r.snapshot.DeleteTranscoder(transcoder)
			// Generate a new envoy snapshot since we have deleted a transcoder.
			if err := r.snapshot.GenerateSnapshot(ctx); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}
	r.snapshot.AddTranscoder(transcoder)
	// Generate a new envoy snapshot since we have added a transcoder.
	if err := r.snapshot.GenerateSnapshot(ctx); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HttpGrpcTranscoderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&HttpGrpcTranscoder{}).
		Complete(r)
}
