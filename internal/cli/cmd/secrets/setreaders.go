// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"bytes"
	"context"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/utils/pointer"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newSetReadersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-readers --from_file {public-key} [server]",
		Short: "Sets the receipients to a secret bundle.",
		Args:  cobra.MaximumNArgs(1),
	}

	fromFile := cmd.Flags().String("from_file", "", "The path of the key file to read.")
	rawtext := cmd.Flags().Bool("rawtext", false, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = cmd.MarkFlagRequired("key")
	env := fncobra.EnvFromValue(cmd, pointer.String("dev"))
	locs := fncobra.LocationsFromArgs(cmd, env)
	loc, bundle := bundleFromArgs(cmd, env, locs, nil)

	return fncobra.With(cmd, func(ctx context.Context) error {
		if *fromFile == "" {
			return fnerrors.New("--file_file is required")
		}

		contents, err := os.ReadFile(*fromFile)
		if err != nil {
			return err
		}

		var keys []string
		lines := bytes.Split(contents, []byte{'\n'})
		for _, line := range lines {
			clean := bytes.TrimSpace(line)
			if len(clean) == 0 || clean[0] == '#' {
				continue
			}
			keys = append(keys, string(clean))
		}

		if err := bundle.SetReaders(keys); err != nil {
			return err
		}

		return writeBundle(ctx, loc, bundle, !*rawtext)
	})
}
