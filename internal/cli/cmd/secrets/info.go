// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [server]",
		Short: "Describes the contents of the specified server's secrets archive.",
		Args:  cobra.MaximumNArgs(1),
	}

	env := envFromValue(cmd, static("dev"))
	locs := locationsFromArgs(cmd, env)
	_, bundle := bundleFromArgs(cmd, env, locs, nil)

	return fncobra.With(cmd, func(ctx context.Context) error {
		bundle.DescribeTo(console.Stdout(ctx))
		return nil
	})
}
