// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
