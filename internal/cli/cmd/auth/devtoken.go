// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewGenerateDevTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-dev-token",
		Short:  "Generate a Namespace Cloud token for development purposes.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the access token to this path.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		res, err := fnapi.IssueDevelopmentToken(ctx)
		if err != nil {
			return err
		}

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(res.DevelopmentToken), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputPath, err)
			}

			return nil
		}

		fmt.Fprintln(console.Stdout(ctx), res.DevelopmentToken)
		return nil
	})
}
