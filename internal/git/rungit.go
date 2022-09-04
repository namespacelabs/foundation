// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

func RunGit(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), NoPromptEnv().Serialize()...)
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	cmd.Dir = dir

	if err := cmd.Run(); err != nil {
		return nil, errOut.Bytes(), err
	}

	return out.Bytes(), errOut.Bytes(), nil
}
