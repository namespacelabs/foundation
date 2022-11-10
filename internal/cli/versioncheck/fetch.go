package versioncheck

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/schema"
)

type Status struct {
	Version    string
	NewVersion bool
	BuildTime  time.Time
}

// Checks for updates and messages from Namespace developers.
// Does nothing if a check for remote status failed
func CheckRemote(ctx context.Context, computeRequirements func(context.Context) (*schema.Workspace_FoundationRequirements, error)) (*Status, error) {
	ver, err := version.Current()
	if err != nil {
		return nil, fnerrors.InternalError("failed to obtain version information: %w", err)
	}

	if ver.BuildTime == nil || ver.Version == version.DevelopmentBuildVersion {
		return nil, nil // Nothing to check.
	}

	var fnReqs *schema.Workspace_FoundationRequirements
	if computeRequirements != nil {
		reqs, err := computeRequirements(ctx)
		if err != nil {
			return nil, fnerrors.InternalError("failed to compute workspace requirements: %w", err)
		}

		fnReqs = reqs
	}

	fmt.Fprintf(console.Debug(ctx), "version check: current %s, build time %v, min API %d\n",
		ver.Version, ver.BuildTime, fnReqs.GetMinimumApi())

	resp, err := fnapi.GetLatestVersion(ctx, fnReqs)
	if err != nil {
		return nil, fnerrors.InternalError("version check failed: %w", err)
	}

	newVersion := semver.Compare(resp.Version, ver.Version) > 0

	fmt.Fprintf(console.Debug(ctx), "version check: got %s, build time: %v, new: %v\n",
		resp.Version, resp.BuildTime, newVersion)

	return &Status{
		Version:    resp.Version,
		BuildTime:  resp.BuildTime,
		NewVersion: newVersion,
	}, nil
}

// XXX this method is not correct; it does not take into account the API requirements of the module's dependencies.
func FetchWorkspaceRequirements(ctx context.Context) (*schema.Workspace_FoundationRequirements, error) {
	moduleRoot, err := cuefrontend.ModuleLoader.FindModuleRoot(".")
	if err != nil {
		// The user is not inside of a workspace. This is normal.
		return nil, nil
	}

	wsData, err := cuefrontend.ModuleLoader.ModuleAt(ctx, moduleRoot)
	if err != nil {
		// Failed to parse workspace. For the purposes of version check it's okay to proceed,
		return nil, err
	}

	return wsData.Proto().Foundation, nil
}
