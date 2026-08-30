// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/std/tasks"
)

func newActionDemoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action-demo",
		Short: "Experiment with console output.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			eg, ctx := errgroup.WithContext(ctx)

			const maxMidDur = 10

			for i := 0; i < 20; i++ {
				i := i
				eg.Go(func() error {
					time.Sleep(time.Duration((rand.Int()+1)%maxMidDur) * time.Second)
					return tasks.Action(fmt.Sprintf("task.%d", i)).Arg("i", i).Run(ctx, func(_ context.Context) error {
						time.Sleep(time.Duration((rand.Int()+1)%maxMidDur) * time.Second)
						fmt.Fprintln(console.Stdout(ctx), "midway", i)
						time.Sleep(time.Duration((rand.Int()+1)%maxMidDur) * time.Second)
						return nil
					})
				})
			}

			return eg.Wait()
		}),
	}

	return cmd
}
