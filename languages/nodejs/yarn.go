// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/workspace"
)

func RunYarn(ctx context.Context, loc workspace.Location, args []string) error {
	var cmd localexec.Command
	cmd.Command = "yarn"
	cmd.Args = args
	cmd.Dir = loc.Rel()
	return cmd.Run(ctx)
}