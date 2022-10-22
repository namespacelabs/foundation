// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/wscontents"
)

func newObserveFsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observe-fs",
		Short: "Continuously observe the filesystem, for debugging purposes.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			module, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			return compute.Continuously(ctx, &sinkObserve{
				events: wscontents.Observe(module.Abs(), ".", true),
			}, nil)
		}),
	}

	return cmd
}

type sinkObserve struct {
	events compute.Computable[wscontents.Versioned]
}

func (so *sinkObserve) Inputs() *compute.In { return compute.Inputs().Computable("events", so.events) }
func (so *sinkObserve) Updated(ctx context.Context, deps compute.Resolved) error {
	fmt.Fprintf(console.Stdout(ctx), "%+v\n", deps)
	return nil
}
func (so *sinkObserve) Cleanup(context.Context) error { return nil }
