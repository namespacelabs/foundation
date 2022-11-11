// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete --secret {package_name}:{secret_name} [server]",
		Short: "Deletes the specified secret value.",
		Args:  cobra.MaximumNArgs(1),
	}

	secretKey := cmd.Flags().String("secret", "", "The secret key, in {package_name}:{name} format.")
	rawtext := cmd.Flags().Bool("rawtext", false, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = cmd.MarkFlagRequired("secret")
	env := envFromValue(cmd, static("dev"))
	locs := locationsFromArgs(cmd, env)
	loc, bundle := bundleFromArgs(cmd, env, locs, nil)

	return fncobra.With(cmd, func(ctx context.Context) error {
		key, err := parseKey(*secretKey, string(loc.packageName))
		if err != nil {
			return err
		}

		if !bundle.Delete(key.PackageName, key.Key) {
			return fnerrors.New("no such key")
		}

		return writeBundle(ctx, loc, bundle, !*rawtext)
	})
}
