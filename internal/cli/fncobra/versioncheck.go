// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/schema"
)

type remoteStatus struct {
	Version    string
	NewVersion bool
	BuildTime  time.Time
}

// Checks for updates and messages from Namespace developers.
// Does nothing if a check for remote status failed
func checkRemoteStatus(ctx context.Context, channel chan remoteStatus) {
	defer close(channel)

	ver, err := version.Current()
	if err != nil {
		fmt.Fprintln(console.Debug(ctx), "failed to obtain version information", err)
		return
	}

	if ver.BuildTime == nil || ver.Version == version.DevelopmentBuildVersion {
		return // Nothing to check.
	}

	fnReqs := getWorkspaceRequirements(ctx)
	minimumApi := 0
	if fnReqs != nil {
		minimumApi = int(fnReqs.MinimumApi)
	}

	fmt.Fprintf(console.Debug(ctx), "version check: current %s, build time %v, min API %d\n",
		ver.Version, ver.BuildTime, minimumApi)

	resp, err := fnapi.GetLatestVersion(ctx, fnReqs)
	if err != nil {
		fmt.Fprintln(console.Debug(ctx), "version check failed:", err)
		return
	}

	newVersion := semver.Compare(resp.Version, ver.Version) > 0

	fmt.Fprintf(console.Debug(ctx), "version check: got %s, build time: %v, new: %v\n",
		resp.Version, resp.BuildTime, newVersion)

	channel <- remoteStatus{
		Version:    resp.Version,
		BuildTime:  resp.BuildTime,
		NewVersion: newVersion,
	}
}

func getWorkspaceRequirements(ctx context.Context) *schema.Workspace_FoundationRequirements {
	moduleRoot, err := cuefrontend.ModuleLoader.FindModuleRoot(".")
	if err != nil {
		// The user is not inside of a workspace. This is normal.
		return nil
	}

	wsData, err := cuefrontend.ModuleLoader.ModuleAt(ctx, moduleRoot)
	if err != nil {
		// Failed to parse workspace. For the purposes of version check it's okay to proceed,
		return nil
	}

	return wsData.Proto().Foundation
}
