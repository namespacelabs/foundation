// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"

	"namespacelabs.dev/foundation/internal/console"
)

func RunGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), NoPromptEnv()...)
	cmd.Stdout = &out
	cmd.Stderr = console.Stderr(ctx)
	cmd.Dir = dir

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}
