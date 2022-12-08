// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type SecretSource interface {
	Load(context.Context, SecretRequest) (*schema.FileContents, error)
	MissingError(refs []*schema.PackageRef, specs []*schema.SecretSpec, servers []schema.PackageName) error
}

type SecretsContext struct {
	WorkspaceModuleName string
	Environment         *schema.Environment
}

type SecretRequest struct {
	SecretsContext SecretsContext
	Server         struct {
		PackageName schema.PackageName
		ModuleName  string
		RelPath     string // Relative path within the module.
	}
	SecretRef *schema.PackageRef
}

type secretUser struct {
	Server  planning.Server
	Secrets []*schema.PackageRef
}

func loadSecrets(ctx context.Context, source SecretSource, env SecretsContext, users ...secretUser) (*runtime.GroundedSecrets, error) {
	return tasks.Return(ctx, tasks.Action("planning.load-secrets"), func(ctx context.Context) (*runtime.GroundedSecrets, error) {
		g := &runtime.GroundedSecrets{}

		var missing []*schema.PackageRef
		var missingSpecs []*schema.SecretSpec
		var missingServer []schema.PackageName
		for _, ps := range users {
			srv := ps.Server

			if len(ps.Secrets) == 0 {
				continue
			}

			specs, err := loadSecretSpecs(ctx, srv.SealedContext(), ps.Secrets...)
			if err != nil {
				return nil, err
			}

			for k, secretRef := range ps.Secrets {
				gsec := runtime.GroundedSecret{
					Ref:  secretRef,
					Spec: specs[k],
				}

				if gsec.Spec.Generate == nil {
					req := SecretRequest{SecretsContext: env, SecretRef: secretRef}
					req.Server.PackageName = srv.Location.PackageName
					req.Server.ModuleName = srv.Location.Module.ModuleName()
					req.Server.RelPath = srv.Location.Rel()
					value, err := source.Load(ctx, req)
					if err != nil {
						return nil, err
					}

					if value == nil {
						missing = append(missing, secretRef)
						missingSpecs = append(missingSpecs, gsec.Spec)
						missingServer = append(missingServer, srv.PackageName())
						continue
					}

					gsec.Value = value
				}

				g.Secrets = append(g.Secrets, gsec)
			}
		}

		if len(missing) > 0 {
			return nil, source.MissingError(missing, missingSpecs, missingServer)
		}

		return g, nil
	})
}

func loadSecretSpecs(ctx context.Context, pl pkggraph.PackageLoader, secrets ...*schema.PackageRef) ([]*schema.SecretSpec, error) {
	var errs []error
	var specs []*schema.SecretSpec // Same indexing as secrets.
	for _, ref := range secrets {
		secretPackage, err := pl.LoadByName(ctx, ref.AsPackageName())
		if err != nil {
			errs = append(errs, err)
		} else {
			if spec := secretPackage.LookupSecret(ref.Name); spec == nil {
				errs = append(errs, fnerrors.NewWithLocation(ref.AsPackageName(), "no such secret %q", ref.Name))
			} else {
				if spec.Generate != nil {
					if spec.Generate.UniqueId == "" {
						errs = append(errs, fnerrors.NewWithLocation(ref.AsPackageName(), "%s: missing unique id", ref.Name))
					} else if spec.Generate.RandomByteCount <= 0 {
						errs = append(errs, fnerrors.NewWithLocation(ref.AsPackageName(), "%s: randomByteCount must be > 0", ref.Name))
					}
				}

				specs = append(specs, spec)
			}
		}
	}

	if err := multierr.New(errs...); err != nil {
		return nil, err
	}

	return specs, nil
}
