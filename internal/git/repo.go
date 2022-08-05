// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package git

import (
	"context"
	"os"
	"strings"
)

func IsRepoRoot(ctx context.Context) (bool, error) {
	out, _, err := RunGit(ctx, ".", "rev-parse", "--show-toplevel")
	if err != nil {
		return false, err
	}

	root := strings.TrimSuffix(string(out), "\n")

	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}

	return cwd == root, nil
}

// E.g. github.com/username/reponame
func RemoteUrl(ctx context.Context) (string, error) {
	out, _, err := RunGit(ctx, ".", "config", "--get", "remote.origin.url")
	if err != nil {
		return "", err
	}

	url := strings.TrimSuffix(string(out), "\n")
	url = strings.TrimSuffix(url, ".git")

	// Trim protocol.
	if parts := strings.SplitN(url, "://", 2); len(parts) == 2 && parts[1] != "" {
		url = parts[1]
	}

	// Trim login.
	if parts := strings.SplitN(url, "@", 2); len(parts) == 2 && parts[1] != "" {
		url = parts[1]
	}

	return url, nil
}
