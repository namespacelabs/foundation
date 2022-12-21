package revision

import (
	"context"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type RevisionReconciler struct {
	client   client.Client
	recorder record.EventRecorder
}

func (r *RevisionReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling", "name", req.NamespacedName)

	// TODO: actually act on the Revision CRD

	return reconcile.Result{}, nil
}
