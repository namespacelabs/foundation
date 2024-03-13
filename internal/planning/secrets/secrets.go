// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type groundedSecrets struct {
	source    secrets.SecretsSource
	sealedCtx pkggraph.SealedPackageLoader
	server    *secrets.SecretLoadRequest_ServerRef
}

type Server interface {
	SealedContext() pkggraph.SealedContext
	PackageName() schema.PackageName
	Module() *pkggraph.Module
	RelPath() string
}

func ScopeSecretsToServer(source secrets.SecretsSource, server Server) secrets.GroundedSecrets {
	return ScopeSecretsTo(source, server.SealedContext(), &secrets.SecretLoadRequest_ServerRef{
		PackageName: server.PackageName(),
		ModuleName:  server.Module().ModuleName(),
		RelPath:     server.RelPath(),
	})
}

func ScopeSecretsTo(source secrets.SecretsSource, sealedCtx pkggraph.SealedPackageLoader, server *secrets.SecretLoadRequest_ServerRef) secrets.GroundedSecrets {
	return groundedSecrets{source: source, sealedCtx: sealedCtx, server: server}
}

type cachedSecret struct {
	groundedSecrets

	ref *schema.PackageRef

	compute.DoScoped[*schema.SecretResult]
}

func (cs *cachedSecret) Action() *tasks.ActionEvent {
	return tasks.Action("read-secret").Arg("ref", cs.ref.Canonical())
}

func (cs *cachedSecret) Inputs() *compute.In {
	return compute.Inputs().Str("ref", cs.ref.Canonical())
}

func (cs *cachedSecret) Compute(ctx context.Context, deps compute.Resolved) (*schema.SecretResult, error) {
	return cs.source.Load(ctx, cs.sealedCtx, &secrets.SecretLoadRequest{SecretRef: cs.ref, Server: cs.server})
}

func (gs groundedSecrets) Get(ctx context.Context, ref *schema.PackageRef) (*schema.SecretResult, error) {
	specs, err := LoadSecretSpecs(ctx, gs.sealedCtx, ref)
	if err != nil {
		return nil, err
	}

	gsec := &schema.SecretResult{
		Ref:  ref,
		Spec: specs[0],
	}

	if gsec.Spec.Generate == nil {
		cs := &cachedSecret{groundedSecrets: gs, ref: ref}
		value, err := compute.GetValue(ctx, cs)
		if err != nil {
			return nil, err
		}

		if value == nil {
			if gsec.Spec.DefaultValue != nil {
				return gsec.Spec.DefaultValue, nil
			}

			var server schema.PackageName
			if gs.server != nil {
				server = gs.server.PackageName
			}
			return nil, gs.source.MissingError(ref, specs[0], server)
		}

		gsec.Value = value.Value
		gsec.FileContents = value.FileContents
		gsec.ExternalRef = value.ExternalRef
	}

	return gsec, nil
}

func LoadSecretSpecs(ctx context.Context, pl pkggraph.PackageLoader, secrets ...*schema.PackageRef) ([]*schema.SecretSpec, error) {
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
