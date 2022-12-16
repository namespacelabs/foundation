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
)

type groundedSecrets struct {
	source    runtime.SecretSource
	sealedCtx pkggraph.SealedPackageLoader
	server    *runtime.SecretRequest_ServerRef
}

func ScopeSecretsToServer(source runtime.SecretSource, server planning.Server) runtime.GroundedSecrets {
	return ScopeSecretsTo(source, server.SealedContext(), &runtime.SecretRequest_ServerRef{
		PackageName: server.PackageName(),
		ModuleName:  server.Module().ModuleName(),
		RelPath:     server.Location.Rel(),
	})
}

func ScopeSecretsTo(source runtime.SecretSource, sealedCtx pkggraph.SealedPackageLoader, server *runtime.SecretRequest_ServerRef) runtime.GroundedSecrets {
	return groundedSecrets{source: source, sealedCtx: sealedCtx, server: server}
}

func (gs groundedSecrets) Get(ctx context.Context, ref *schema.PackageRef) (*schema.SecretResult, error) {
	specs, err := loadSecretSpecs(ctx, gs.sealedCtx, ref)
	if err != nil {
		return nil, err
	}

	gsec := &schema.SecretResult{
		Ref:  ref,
		Spec: specs[0],
	}

	if gsec.Spec.Generate == nil {
		req := runtime.SecretRequest{SecretRef: ref, Server: gs.server}
		req.Server = gs.server
		value, err := gs.source.Load(ctx, gs.sealedCtx, req)
		if err != nil {
			return nil, err
		}

		if value == nil {
			var server schema.PackageName
			if gs.server != nil {
				server = gs.server.PackageName
			}
			return nil, gs.source.MissingError(ref, specs[0], server)
		}

		gsec.Value = value
	}

	return gsec, nil
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
