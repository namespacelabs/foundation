// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/workspace/compute"
)

func RunE(f func(ctx context.Context, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx, cancel := WithSigIntCancel(cmd.Context())
		defer cancel()

		return compute.Do(ctx, func(ctx context.Context) error {
			return f(ctx, args)
		})
	}
}
