// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
)

const (
	defaultMetadataSpecFile = "/var/run/nsc/token.spec"
)

var supportedMetadataKeys = []string{"id"}

func NewMetadataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Interact with instance metadata.",
	}

	cmd.AddCommand(newReadCmd())

	return cmd
}

func newReadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read [metadata-key]",
		Short: "Read a metadata value for the provided key.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		metadataKey := args[0]
		if !slices.Contains(supportedMetadataKeys, metadataKey) {
			return fnerrors.Newf("reading instance metadata value for the key %q is not supported", metadataKey)
		}

		spec, err := fnapi.ResolveSpec()
		if err != nil {
			return err
		}

		if spec == "" {
			s, err := os.ReadFile(defaultMetadataSpecFile)
			if err != nil {
				return fnerrors.Newf("failed to read metadata spec file: %w", err)
			}

			spec = string(s)
		}

		value, err := metadata.FetchValueFromSpec(ctx, spec, metadataKey)
		if err != nil {
			return err
		}

		fmt.Fprintln(console.Stdout(ctx), value)
		return nil
	})

	return cmd
}
