// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/cli/version"
)

// Checks for updates and messages from Namespace developers.
// Does nothing if a check for remote status failed
func checkRemoteStatus(debugLogger io.Writer, channel chan remoteStatus) {
	defer close(channel)

	ver, err := version.Version()
	if err != nil {
		fmt.Fprintln(debugLogger, "failed to obtain version information", err)
		return
	}

	if ver.BuildTime == nil || ver.Version == version.DevelopmentBuildVersion {
		return // Nothing to check.
	}

	fmt.Fprintln(debugLogger, "version to check:", ver.BuildTime)

	status, err := FetchLatestRemoteStatus(context.Background(), versionCheckEndpoint, ver.GitCommit)
	if err != nil {
		fmt.Fprintln(debugLogger, "version check failed:", err)
	} else {
		fmt.Fprintln(debugLogger, "version check:", status.BuildTime)

		if semver.Compare(status.Version, ver.Version) > 0 {
			status.NewVersion = true
		}

		channel <- *status
	}
}
