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
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
)

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
	root *parsing.Root
	loc  pkggraph.Location
}

func loadBundleFromArgs(ctx context.Context, env planning.Context, loc fnfs.Location, createIfMissing createFunc) (*location, *secrets.Bundle, error) {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return nil, nil, err
	}

	pkg, err := parsing.NewPackageLoader(env).LoadByName(ctx, loc.AsPackageName())
	if err != nil {
		return nil, nil, err
	}

	if !isModuleRoot(pkg.Location) && pkg.Server == nil {
		return nil, nil, fnerrors.BadInputError("%s: expected a server or a workspace root", loc.AsPackageName())
	}

	contents, err := fs.ReadFile(pkg.Location.Module.ReadWriteFS(), secretBundle(pkg.Location))
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

func parseKey(v string, defaultPkgName string) (*secrets.ValueKey, error) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) < 2 {
		parts = []string{defaultPkgName, parts[0]}
	}

	return &secrets.ValueKey{PackageName: parts[0], Key: parts[1]}, nil
}

func writeBundle(ctx context.Context, loc *location, bundle *secrets.Bundle, encrypt bool) error {
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), loc.root.ReadWriteFS(), secretBundle(loc.loc), func(w io.Writer) error {
		return bundle.SerializeTo(ctx, w, encrypt)
	})
}

func secretBundle(loc pkggraph.Location) string {
	if isModuleRoot(loc) {
		return secrets.WorkspaceBundleName
	}

	return loc.Rel(secrets.ServerBundleName)
}

func isModuleRoot(loc pkggraph.Location) bool {
	return loc.PackageName == loc.Module.RootLocation().PackageName
}
