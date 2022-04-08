// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"namespacelabs.dev/foundation/internal/localexec"
)

func RunYarn(ctx context.Context, relPath string, args []string) error {
	var cmd localexec.Command
	cmd.Command = "yarn"
	cmd.Args = args
	cmd.Dir = relPath
	return cmd.Run(ctx)
}
