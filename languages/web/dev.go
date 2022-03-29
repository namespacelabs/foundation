// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func StartDevServer(ctx context.Context, root *workspace.Root, pkg schema.PackageName, mainPort, webPort int64) (string, error) {
	host := "127.0.0.1"
	hostPort := fmt.Sprintf("%s:%d", host, webPort)

	loc, err := workspace.NewPackageLoader(root).Resolve(ctx, pkg)
	if err != nil {
		return "", err
	}

	go func() {
		var cmd localexec.Command
		cmd.Label = "vite"
		cmd.Command = "node_modules/.bin/vite"
		cmd.Args = []string{"--config=devweb.config.js", "--clearScreen=false", "--host=" + host, fmt.Sprintf("--port=%d", webPort)}
		cmd.Dir = loc.Abs()
		cmd.AdditionalEnv = []string{fmt.Sprintf("CMD_DEV_PORT=%d", mainPort)}
		cmd.Persistent = true
		cmd.Run(ctx)
	}()

	return hostPort, nil
}