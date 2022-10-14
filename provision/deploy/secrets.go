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
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func loadSecrets(ctx context.Context, env *schema.Environment, stack *provision.Stack) (*runtime.GroundedSecrets, error) {
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
	var missingServer []schema.PackageName
	for _, ps := range stack.Servers {
		srv := ps.Server
		if len(srv.Proto().SecretRefs) == 0 {
			continue
		}

		if _, has := workspaceSecrets[srv.Module().ModuleName()]; !has {
			wss, err := loadWorkspaceSecrets(ctx, keyDir, srv.Module())
			if err != nil {
				if !errors.Is(err, keys.ErrKeyGen) {
					return nil, err
				}
			} else {
				workspaceSecrets[srv.Module().ModuleName()] = wss
			}
		}

		serverBundle, err := getServerBundle(ctx, srv, keyDir)
		if err != nil {
			return nil, err
		}

		for _, secretRef := range srv.Proto().SecretRefs {
			if !strings.HasPrefix(secretRef.PackageName, srv.Module().ModuleName()) {
				return nil, fnerrors.InternalError("%s: secret %q is not in the same module as the server, which is not supported yet", srv.PackageName(), secretRef.PackageName)
			}

			value, err := lookupSecret(ctx, env, secretRef, serverBundle, workspaceSecrets[srv.Module().ModuleName()])
			if err != nil {
				return nil, err
			}

			if value == nil {
				missing = append(missing, secretRef)
				missingServer = append(missingServer, srv.PackageName())
				continue
			}

			g.Secrets = append(g.Secrets, runtime.GroundedSecret{
				Ref:   secretRef,
				Value: value,
			})
		}
	}

	if len(missing) > 0 {
		labels := make([]string, len(missing))

		for k, secretRef := range missing {
			labels[k] = fmt.Sprintf("  ns secrets set %s --secret %s", missingServer[k], secretRef.Canonical())
		}

		return nil, fnerrors.UsageError(
			fmt.Sprintf("Please run:\n\n%s", strings.Join(labels, "\n")),
			"there are secrets required which have not been specified")

	}

	return g, nil
}

func getServerBundle(ctx context.Context, srv parsed.Server, keyDir fnfs.LocalFS) (*secrets.Bundle, error) {
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

		return &schema.FileContents{Contents: value, Utf8: true}, nil
	}

	if workspace != nil {
		value, err := workspace.Lookup(ctx, key)
		if err != nil {
			return nil, err
		}

		return &schema.FileContents{Contents: value, Utf8: true}, nil
	}

	return nil, nil
}
