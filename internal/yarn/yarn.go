// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package yarn

import (
	"context"

	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func RunYarn(ctx context.Context, relPath string, args []string) error {
	var cmd localexec.Command
	cmd.Command = "yarn"
	cmd.Args = args
	cmd.Dir = relPath
	fnModuleCache, err := dirs.ModuleCacheRoot()
	if err != nil {
		return err
	}
	cmd.AdditionalEnv = []string{"FN_MODULE_CACHE=" + fnModuleCache}
	return cmd.Run(ctx)
}
