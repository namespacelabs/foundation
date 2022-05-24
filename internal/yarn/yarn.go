// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package yarn

import (
	"context"

	"namespacelabs.dev/foundation/internal/localexec"
	yarnsdk "namespacelabs.dev/foundation/internal/sdk/yarn"
)

// Runs Yarn v3+ for Node.js
func RunYarn(ctx context.Context, relPath string, args []string, env []string) error {
	bin, err := yarnsdk.EnsureSDK(ctx)
	if err != nil {
		return err
	}

	var cmd localexec.Command
	cmd.Command = "node"
	cmd.Args = append([]string{string(bin)}, args...)
	cmd.Dir = relPath
	cmd.AdditionalEnv = env

	return cmd.Run(ctx)
}

// Runs Yarn v1+ for Web.
func RunYarnV1(ctx context.Context, relPath string, args []string) error {
	var cmd localexec.Command
	cmd.Command = "yarn"
	cmd.Args = args
	cmd.Dir = relPath

	return cmd.Run(ctx)
}
