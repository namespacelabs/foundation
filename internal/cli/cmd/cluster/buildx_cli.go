// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

// We drive the user's host-installed `docker buildx` CLI to manage builder
// instances, rather than depending on github.com/docker/buildx as a library
// (which in turn pulls in the deprecated github.com/docker/docker module). The
// CLI owns the on-disk store format, so this guarantees compatibility with
// whatever buildx the user has installed.

func runDockerBuildx(ctx context.Context, args ...string) (string, error) {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return "", fnerrors.Newf("the 'docker' CLI is required to manage Namespace remote builders but was not found in PATH: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, dockerBin, append([]string{"buildx"}, args...)...)
	cmd.Stdout = io.MultiWriter(&stdout, console.Debug(ctx))
	cmd.Stderr = io.MultiWriter(&stderr, console.Debug(ctx))

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fnerrors.Newf("docker buildx %s failed: %s", strings.Join(args, " "), msg)
	}

	return stdout.String(), nil
}

// buildxCreateNode registers (or, with appendNode, appends) a single node into
// the named builder. It mirrors the previous store.NodeGroup.Update call.
func buildxCreateNode(ctx context.Context, name, driver, nodeName, endpoint string, platforms []string, driverOpts map[string]string, appendNode bool) error {
	args := []string{"create", "--name", name, "--node", nodeName}
	if appendNode {
		args = append(args, "--append")
	} else {
		args = append(args, "--driver", driver)
	}

	for _, p := range platforms {
		args = append(args, "--platform", p)
	}

	// Sort keys so the generated command line is deterministic.
	keys := make([]string, 0, len(driverOpts))
	for k := range driverOpts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--driver-opt", fmt.Sprintf("%s=%s", k, driverOpts[k]))
	}

	args = append(args, endpoint)

	_, err := runDockerBuildx(ctx, args...)
	return err
}

// buildxUse sets the named builder as the current one for the active context.
func buildxUse(ctx context.Context, name string) error {
	_, err := runDockerBuildx(ctx, "use", name)
	return err
}

// buildxRemove removes the named builder. It returns an error if the builder
// does not exist; callers that treat removal as best-effort should ignore it.
func buildxRemove(ctx context.Context, name string) error {
	_, err := runDockerBuildx(ctx, "rm", "--force", name)
	return err
}

// readBuildxMetadata reads our own state file describing the builders we set
// up. The boolean reports whether the file exists.
func readBuildxMetadata(stateDir string) (buildxMetadata, bool, error) {
	var md buildxMetadata
	if err := files.ReadJson(filepath.Join(stateDir, metadataFile), &md); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return md, false, nil
		}
		return md, false, err
	}

	return md, true, nil
}
