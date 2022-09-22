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
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/provision"
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

	var missing []*schema.SecretSpec
	var missingServer []schema.PackageName
	for _, ps := range stack.Servers {
		srv := ps.Server
		if len(srv.Proto().Secret) == 0 {
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

		contents, err := fs.ReadFile(srv.Location.Module.ReadOnlyFS(), srv.Location.Rel(secrets.ServerBundleName))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fnerrors.InternalError("%s: failed to read %q: %w", srv.PackageName(), secrets.ServerBundleName, err)
		}

		bundle, err := secrets.LoadBundle(ctx, keyDir, contents)
		if err != nil {
			if !errors.Is(err, keys.ErrKeyGen) {
				return nil, err
			}
		}

		for _, secret := range srv.Proto().Secret {
			value, err := lookupSecret(ctx, env, secret, bundle, workspaceSecrets[srv.Module().ModuleName()])
			if err != nil {
				return nil, err
			}

			if value == nil {
				if !secret.Optional {
					missing = append(missing, secret)
					missingServer = append(missingServer, srv.PackageName())
				}
				continue
			}

			g.Secrets = append(g.Secrets, runtime.GroundedSecret{
				Owner: schema.PackageName(secret.Owner),
				Name:  secret.Name,
				Value: value,
			})
		}
	}

	if len(missing) > 0 {
		labels := make([]string, len(missing))

		for k, secret := range missing {
			labels[k] = fmt.Sprintf("  ns secrets set %s --secret %s:%s", missingServer[k], secret.Owner, secret.Name)
		}

		return nil, fnerrors.UsageError(
			fmt.Sprintf("Please run:\n\n%s", strings.Join(labels, "\n")),
			"there are secrets required which have not been specified")

	}

	return g, nil
}

func lookupSecret(ctx context.Context, env *schema.Environment, secret *schema.SecretSpec, server, workspace *secrets.Bundle) (*schema.FileContents, error) {
	key := &secrets.ValueKey{PackageName: secret.Owner, Key: secret.Name, EnvironmentName: env.Name}

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
