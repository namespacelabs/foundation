// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type secretSource struct {
	Server  planning.Server
	Secrets []*schema.PackageRef
}

func loadSecrets(ctx context.Context, env *schema.Environment, sources ...secretSource) (*runtime.GroundedSecrets, error) {
	return tasks.Return(ctx, tasks.Action("planning.load-secrets"), func(ctx context.Context) (*runtime.GroundedSecrets, error) {
		keyDir, err := keys.KeysDir()
		if err != nil {
			if errors.Is(err, keys.ErrKeyGen) {
				keyDir = nil
			} else {
				return nil, err
			}
		}

		workspaceSecrets := map[string]*secrets.Bundle{}

		g := &runtime.GroundedSecrets{}

		var missing []*schema.PackageRef
		var missingSpecs []*schema.SecretSpec
		var missingServer []schema.PackageName
		for _, ps := range sources {
			srv := ps.Server

			if len(ps.Secrets) == 0 {
				continue
			}

			var errs []error
			var specs []*schema.SecretSpec // Same indexing as secrets.
			for _, ref := range ps.Secrets {
				secretPackage, err := srv.SealedContext().LoadByName(ctx, ref.AsPackageName())
				if err != nil {
					errs = append(errs, err)
				} else {
					if spec := secretPackage.LookupSecret(ref.Name); spec == nil {
						errs = append(errs, fnerrors.UserError(ref.AsPackageName(), "no such secret %q", ref.Name))
					} else {
						if spec.Generate != nil {
							if spec.Generate.UniqueId == "" {
								errs = append(errs, fnerrors.UserError(ref.AsPackageName(), "%s: missing unique id", ref.Name))
							} else if spec.Generate.RandomByteCount <= 0 {
								errs = append(errs, fnerrors.UserError(ref.AsPackageName(), "%s: randomByteCount must be > 0", ref.Name))
							}
						}

						specs = append(specs, spec)
					}
				}
			}

			if err := multierr.New(errs...); err != nil {
				return nil, err
			}

			if _, has := workspaceSecrets[srv.Module().ModuleName()]; !has {
				wss, err := loadWorkspaceSecrets(ctx, keyDir, srv.Module())
				if err != nil {
					// XXX print a warning?
					if !errors.Is(err, keys.ErrKeyGen) {
						return nil, err
					}
				} else {
					workspaceSecrets[srv.Module().ModuleName()] = wss
				}
			}

			srvSecrets, err := loadServerSecrets(ctx, keyDir, srv)
			if err != nil {
				return nil, err
			}

			for k, secretRef := range ps.Secrets {
				gsec := runtime.GroundedSecret{
					Ref:  secretRef,
					Spec: specs[k],
				}

				if gsec.Spec.Generate == nil {
					value, err := lookupSecret(ctx, env, secretRef, srvSecrets, workspaceSecrets[srv.Module().ModuleName()])
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
			labels := make([]string, len(missing))

			for k, secretRef := range missing {
				labels[k] = fmt.Sprintf("  # Description: %s\n  ns secrets set %s --secret %s", missingSpecs[k].Description, missingServer[k], secretRef.Canonical())
			}

			return nil, fnerrors.UsageError(
				fmt.Sprintf("Please run:\n\n%s", strings.Join(labels, "\n")),
				"There are secrets required which have not been specified")

		}

		return g, nil
	})
}

func loadWorkspaceSecrets(ctx context.Context, keyDir fs.FS, module *pkggraph.Module) (*secrets.Bundle, error) {
	contents, err := fs.ReadFile(module.ReadOnlyFS(), secrets.WorkspaceBundleName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fnerrors.InternalError("%s: failed to read %q: %w", module.Workspace.ModuleName, secrets.WorkspaceBundleName, err)
	}

	return secrets.LoadBundle(ctx, keyDir, contents)
}

func loadServerSecrets(ctx context.Context, keyDir fnfs.LocalFS, srv planning.Server) (*secrets.Bundle, error) {
	contents, err := fs.ReadFile(srv.Location.Module.ReadOnlyFS(), srv.Location.Rel(secrets.ServerBundleName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fnerrors.InternalError("%s: failed to read %q: %w", srv.PackageName(), secrets.ServerBundleName, err)
	}

	bundle, err := secrets.LoadBundle(ctx, keyDir, contents)
	if err != nil {
		if !errors.Is(err, keys.ErrKeyGen) {
			return nil, err
		}
	}
	return bundle, nil
}

func lookupSecret(ctx context.Context, env *schema.Environment, secretRef *schema.PackageRef, server, workspace *secrets.Bundle) (*schema.FileContents, error) {
	key := &secrets.ValueKey{PackageName: secretRef.PackageName, Key: secretRef.Name, EnvironmentName: env.Name}

	if server != nil {
		value, err := server.Lookup(ctx, key)
		if err != nil {
			return nil, err
		}

		if value != nil {
			return &schema.FileContents{Contents: value, Utf8: true}, nil
		}
	}

	if workspace != nil {
		value, err := workspace.Lookup(ctx, key)
		if err != nil {
			return nil, err
		}

		if value != nil {
			return &schema.FileContents{Contents: value, Utf8: true}, nil
		}
	}

	return nil, nil
}
