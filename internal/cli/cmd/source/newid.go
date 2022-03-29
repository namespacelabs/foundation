// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/go-ids"
)

var (
	n      = 12
	base62 = false
)

func newNewIdCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new-id",
		Short: "Generate a new ID.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			stdout := console.Stdout(ctx)
			fmt.Fprintln(stdout, newId())
			return nil
		}),
	}

	cmd.Flags().IntVar(&n, "len", n, "Number of bytes.")
	cmd.Flags().BoolVar(&base62, "base62", base62, "Generate a base62-based random ID.")

	return cmd
}

func newId() string {
	if base62 {
		return ids.NewRandomBase62ID(n)
	} else {
		return ids.NewRandomBase32ID(n)
	}
}