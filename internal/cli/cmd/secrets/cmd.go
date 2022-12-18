// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/secrets/localsecrets"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage secrets for the current workspace or a given server.",
	}

	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newRevealCmd())
	cmd.AddCommand(newAddReaderCmd())
	cmd.AddCommand(newSetReadersCmd())

	return cmd
}

type createFunc func(context.Context) (*localsecrets.Bundle, error)

type location struct {
	workspaceFS fnfs.ReadWriteFS
	sourceFile  string
}

func bundleFromArgs(cmd *cobra.Command, env *cfg.Context, locs *fncobra.Locations, createIfMissing createFunc) (*location, *localsecrets.Bundle) {
	targetloc := new(location)
	targetbundle := new(localsecrets.Bundle)

	user := cmd.Flags().Bool("user", false, "If set, updates a user-owned secret database which can be more easily git-ignored.")

	pushParse(cmd, func(ctx context.Context, args []string) error {
		loc, bundle, err := loadBundleFromArgs(ctx, *env, *locs, *user, createIfMissing)
		if err != nil {
			return err
		}
		*targetloc = *loc
		*targetbundle = *bundle
		return nil
	})

	return targetloc, targetbundle
}

func loadBundleFromArgs(ctx context.Context, env cfg.Context, locs fncobra.Locations, user bool, createIfMissing createFunc) (*location, *localsecrets.Bundle, error) {
	if env.Workspace().LoadedFrom() == nil {
		return nil, nil, fnerrors.InternalError("workspace is missing it's source")
	}

	workspaceFS := fnfs.ReadWriteLocalFS(env.Workspace().LoadedFrom().AbsPath)
	result := &location{workspaceFS: workspaceFS}

	switch len(locs.Locs) {
	case 0:
		// Workspace
		if user {
			result.sourceFile = localsecrets.UserBundleName
		} else {
			result.sourceFile = localsecrets.WorkspaceBundleName
		}

	case 1:
		loc := locs.Locs[0]

		if user {
			return nil, nil, fnerrors.New("can use --user and %q at the same time", loc.AsPackageName())
		}

		pkg, err := parsing.NewPackageLoader(env).LoadByName(ctx, loc.AsPackageName())
		if err != nil {
			return nil, nil, err
		}

		if pkg.Server == nil {
			return nil, nil, fnerrors.BadInputError("%s: expected a server", loc.AsPackageName())
		}

		result.sourceFile = loc.Rel(localsecrets.ServerBundleName)

	default:
		return nil, nil, fnerrors.New("expected up to a single package to be selected, saw %d", len(locs.Locs))
	}

	contents, err := fs.ReadFile(workspaceFS, result.sourceFile)
	if err != nil {
		if os.IsNotExist(err) && createIfMissing != nil {
			bundle, err := createIfMissing(ctx)
			return result, bundle, err
		}

		return nil, nil, err
	}

	keyDir, err := keys.KeysDir()
	if err != nil {
		return nil, nil, err
	}

	bundle, err := localsecrets.LoadBundle(ctx, keyDir, contents)
	return result, bundle, err
}

func parseKey(v string) (*localsecrets.ValueKey, error) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) < 2 {
		return nil, fnerrors.New("expected secret format to be {package_name}:{name}")
	}

	return &localsecrets.ValueKey{PackageName: parts[0], Key: parts[1]}, nil
}

func writeBundle(ctx context.Context, loc *location, bundle *localsecrets.Bundle, encrypt bool) error {
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), loc.workspaceFS, loc.sourceFile, func(w io.Writer) error {
		return bundle.SerializeTo(ctx, w, encrypt)
	})
}
