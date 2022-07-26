// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

const bundleName = "server.secrets"

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage secrets for a given server.",
	}

	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newRevealCmd())
	cmd.AddCommand(newAddReaderCmd())

	return cmd
}

type createFunc func(context.Context) (*secrets.Bundle, error)

type location struct {
	root *workspace.Root
	loc  workspace.Location
}

func loadBundleFromArgs(ctx context.Context, locs *fncobra.Locations, createIfMissing createFunc) (*location, *secrets.Bundle, error) {
	loc, err := locs.AssertSingle()
	if err != nil {
		return nil, nil, err
	}

	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return nil, nil, err
	}

	pkg, err := workspace.NewPackageLoader(root).LoadByName(ctx, loc.AsPackageName())
	if err != nil {
		return nil, nil, err
	}

	if pkg.Server == nil {
		return nil, nil, fnerrors.BadInputError("%s: expected a server", loc.AsPackageName())
	}

	contents, err := fs.ReadFile(pkg.Location.Module.ReadWriteFS(), pkg.Location.Rel(bundleName))
	if err != nil {
		if os.IsNotExist(err) && createIfMissing != nil {
			bundle, err := createIfMissing(ctx)
			return &location{root, pkg.Location}, bundle, err
		}

		return nil, nil, err
	}

	keyDir, err := keys.KeysDir()
	if err != nil {
		return nil, nil, err
	}

	bundle, err := secrets.LoadBundle(ctx, keyDir, contents)
	return &location{root, pkg.Location}, bundle, err
}

func parseKey(v string) (*secrets.ValueKey, error) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) < 2 {
		return nil, fnerrors.BadInputError("invalid secret key definition, expected {package_name}:{name}")
	}

	return &secrets.ValueKey{PackageName: parts[0], Key: parts[1]}, nil
}

func writeBundle(ctx context.Context, loc *location, bundle *secrets.Bundle, encrypt bool) error {
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), loc.root.FS(), loc.loc.Rel(bundleName), func(w io.Writer) error {
		return bundle.SerializeTo(ctx, w, encrypt)
	})
}
