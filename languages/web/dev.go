// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func StartDevServer(ctx context.Context, env provision.Env, pkg schema.PackageName, mainPort, webPort int) (string, error) {
	host := "127.0.0.1"
	hostPort := fmt.Sprintf("%s:%d", host, webPort)

	loc, err := workspace.NewPackageLoader(env, env.Proto()).Resolve(ctx, pkg)
	if err != nil {
		return "", err
	}

	go func() {
		var cmd localexec.Command
		cmd.Label = "vite"
		cmd.Command = "node_modules/vite/bin/vite.js"
		cmd.Args = []string{"--config=devweb.config.js", "--clearScreen=false", "--host=" + host, fmt.Sprintf("--port=%d", webPort)}
		cmd.Dir = loc.Abs()
		cmd.AdditionalEnv = []string{fmt.Sprintf("CMD_DEV_PORT=%d", mainPort)}
		cmd.Persistent = true
		if err := cmd.Run(ctx); err != nil {
			fmt.Fprintf(console.Warnings(ctx), "vite failed: %v\n", err)
		}
	}()

	return hostPort, nil
}
