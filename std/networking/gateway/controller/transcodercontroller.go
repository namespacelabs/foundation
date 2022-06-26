// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	AppliedCondition = "Applied"
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

	// Preserve all conditions for the transcoder except "Applied".
	var conditions []metav1.Condition
	for _, c := range transcoder.Status.Conditions {
		if c.Type != AppliedCondition {
			conditions = append(conditions, c)
		}
	}
	appliedCondition := metav1.Condition{
		Type:               AppliedCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: transcoder.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	}

	r.snapshot.AddTranscoder(transcoder)

	// Generate a new envoy snapshot since we have added a transcoder.
	snapshotErr := r.snapshot.GenerateSnapshot(ctx)

	// Update the applied condition if we have an error generating the snapshot.
	if snapshotErr != nil {
		appliedCondition.Status = metav1.ConditionFalse
		appliedCondition.Reason = "FailedToGenerateSnapshot"
		appliedCondition.Message = snapshotErr.Error()
	}
	conditions = append(conditions, appliedCondition)
	transcoder.Status.Conditions = conditions

	// Update the status condition on the transcoder.
	if updateErr := r.Client.Status().Update(ctx, transcoder); updateErr != nil {
		// Requeue (rate-limited) if we lost an update race.
		if apierrors.IsConflict(updateErr) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{}, snapshotErr
}

func (r *HttpGrpcTranscoderReconciler) SetupWithManager(mgr ctrl.Manager, matchNamespace string) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&HttpGrpcTranscoder{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				return object.GetNamespace() == matchNamespace
			},
		))).
		Complete(r)
}
