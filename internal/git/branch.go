// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package git

import (
	"context"
	"strings"
)

func CurrentBranch(ctx context.Context, path string) (string, error) {
	out, _, err := RunGit(ctx, path, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}

	branch := strings.TrimSuffix(string(out), "\n")
	return branch, nil
}
