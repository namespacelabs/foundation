// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package versioncheck

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/storage"
)

type Status struct {
	Version    string
	NewVersion bool
	BuildTime  time.Time
}

// Checks for updates and messages from Namespace developers.
// Does nothing if a check for remote status failed
func CheckRemote(ctx context.Context, current *storage.NamespaceBinaryVersion, command string) (*Status, error) {
	resp, err := fnapi.GetLatestVersion(ctx, map[string]any{command: map[string]any{}})
	if err != nil {
		return nil, fnerrors.InternalError("version check failed: %w", err)
	}

	newVersion := semver.Compare(resp.Version, current.Version) > 0

	fmt.Fprintf(console.Debug(ctx), "version check: got %s, build time: %v, new: %v\n",
		resp.Version, resp.BuildTime, newVersion)

	return &Status{
		Version:    resp.Version,
		BuildTime:  resp.BuildTime,
		NewVersion: newVersion,
	}, nil
}
