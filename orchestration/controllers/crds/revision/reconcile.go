// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package revision

import (
	"context"
	"io"
	"io/fs"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/client-go/tools/record"
	orchproto "namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/std/tasks"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/providers/aws/ecr"
	"namespacelabs.dev/foundation/schema"
)

const (
	revisionFinalizerKey = "revisions.k8s.namespacelabs.dev/finalizer"
)

type RevisionReconciler struct {
	clt      client.Client
	recorder record.EventRecorder
	orchClt  orchproto.OrchestrationServiceClient
}

func (r *RevisionReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling", "name", req.NamespacedName)

	rev := &Revision{}
	err := r.clt.Get(ctx, req.NamespacedName, rev)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if rev.ObjectMeta.DeletionTimestamp.IsZero() {
		// Revision is *not* under deletion - i.e. create or update
		if !controllerutil.ContainsFinalizer(rev, revisionFinalizerKey) {
			// Let's add the finalizer key if object does not have it
			controllerutil.AddFinalizer(rev, revisionFinalizerKey)
			if err := r.clt.Update(ctx, rev); err != nil {
				return reconcile.Result{}, err
			}
		}
		if err := r.handleCreateUpdate(ctx, rev); err != nil {
			// if failed, return with error so that it can be retried
			return reconcile.Result{}, err
		}
	} else {
		// Revision is under deletion
		if controllerutil.ContainsFinalizer(rev, revisionFinalizerKey) {
			// our finalizer is present, so let's process the deletion
			if err := r.handleDelete(ctx, rev); err != nil {
				// if failed, return with error so that it can be retried
				return reconcile.Result{}, err
			}

			// remove our finalizer from the list and update it - so API server can delete the object
			controllerutil.RemoveFinalizer(rev, revisionFinalizerKey)
			if err := r.clt.Update(ctx, rev); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *RevisionReconciler) handleCreateUpdate(ctx context.Context, rev *Revision) error {
	log := log.FromContext(ctx)
	plan, err := loadPlan(ctx, rev.Spec.Image)
	if err != nil {
		log.Error(err, "failed to load plan")
		return err
	}

	resp, err := r.orchClt.Deploy(ctx, &orchproto.DeployRequest{
		Plan: plan,
	})
	if err != nil {
		return err
	}

	log.Info("deployed plan", "deploy ID", resp.Id)
	return nil
}

func (r *RevisionReconciler) handleDelete(ctx context.Context, rev *Revision) error {
	log := log.FromContext(ctx)

	//TODO: handle undeploy use-case

	log.Info("delete case")
	return nil
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
	imageID, err := oci.ParseImageID(path)
	if err != nil {
		return nil, err
	}

	var img oci.Image
	ctx = tasks.WithSink(ctx, tasks.NullSink())
	err = compute.Do(ctx, func(ctx context.Context) error {
		img, err = compute.GetValue(ctx, oci.ImageP(imageID.ImageRef(), nil, oci.ResolveOpts{
			PublicImage: false,
			RegistryAccess: oci.RegistryAccess{
				InsecureRegistry: false,
				Keychain:         ecr.DefaultKeychain, //TODO: hard-coded ECR authenticator? What about other registries?
			},
		}))
		return err
	})
	if err != nil {
		return nil, err
	}

	fsys := tarfs.FS{TarStream: func() (io.ReadCloser, error) { return mutate.Extract(img), nil }}

	return fs.ReadFile(fsys, "deployplan.binarypb")
}
