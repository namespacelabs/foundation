package revision

import (
	"context"
	"io"
	"io/fs"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/schema"
)

type RevisionReconciler struct {
	clt      client.Client
	recorder record.EventRecorder
}

func (r *RevisionReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling", "name", req.NamespacedName)

	var rev Revision
	err := r.clt.Get(ctx, req.NamespacedName, &rev)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// TODO: handle delete use-case
			return reconcile.Result{}, nil
		}
		log.Error(err, "failed to get Revision")
		return reconcile.Result{}, err
	}

	plan, err := loadPlan(ctx, rev.Spec.Image)
	if err != nil {
		log.Error(err, "failed to load plan")
		return reconcile.Result{Requeue: true}, err
	}

	log.Info("got plan", "plan", plan.String())

	return reconcile.Result{}, nil
}

func loadPlan(ctx context.Context, path string) (*schema.DeployPlan, error) {
	raw, err := loadPlanContents(ctx, path)
	if err != nil {
		return nil, fnerrors.New("failed to load %q: %w", path, err)
	}

	any := &anypb.Any{}
	if err := proto.Unmarshal(raw, any); err != nil {
		return nil, fnerrors.New("failed to unmarshal %q: %w", path, err)
	}

	plan := &schema.DeployPlan{}
	if err := any.UnmarshalTo(plan); err != nil {
		return nil, fnerrors.New("failed to unmarshal %q: %w", path, err)
	}

	return plan, nil
}

func loadPlanContents(ctx context.Context, path string) ([]byte, error) {
	image, err := compute.GetValue(ctx, oci.ImageP(path, nil, oci.ResolveOpts{}))
	if err != nil {
		return nil, err
	}

	fsys := tarfs.FS{TarStream: func() (io.ReadCloser, error) { return mutate.Extract(image), nil }}

	return fs.ReadFile(fsys, "deployplan.binarypb")
}
