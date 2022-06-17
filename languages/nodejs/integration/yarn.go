// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"context"
	"path/filepath"

	yarnsdk "namespacelabs.dev/foundation/internal/sdk/yarn"
	"namespacelabs.dev/foundation/internal/yarn"
	"namespacelabs.dev/foundation/workspace"
)

const (
	FnYarnLockEnvVar = "FN_YARN_LOCK_FILENAME"
)

// Runs a configured Yarn.
// TODO: move to a shared place, both nodejs and web integrations use this.
func RunNodejsYarn(ctx context.Context, relPath string, args []string, workspaceData workspace.WorkspaceData) error {
	yarnAuxDir, err := yarnsdk.EnsureYarnAuxFilesDir(ctx)
	if err != nil {
		return err
	}

	envArgs := []string{}
	for k, v := range YarnEnvArgs(yarnAuxDir) {
		envArgs = append(envArgs, k+"="+v)
	}
	lockFn, err := writeLockFileToTemp(workspaceData)
	if err != nil {
		return err
	}
	envArgs = append(envArgs, FnYarnLockEnvVar+"="+lockFn)

	return yarn.RunYarn(ctx, relPath, args, envArgs)
}

func YarnEnvArgs(root string) map[string]string {
	return map[string]string{
		"YARN_PLUGINS":     filepath.Join(root, yarnsdk.PluginFn),
		"YARN_RC_FILENAME": filepath.Join(root, yarnsdk.YarnRcFn),
	}
}
