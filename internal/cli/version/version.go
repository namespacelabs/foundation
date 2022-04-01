// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package version

import (
	"runtime/debug"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type BinaryVersion struct {
	Version      string
	BuildTimeStr string
	BuildTime    *time.Time
	Modified     bool
}

func Version() (*BinaryVersion, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, fnerrors.InternalError("buildinfo is missing")
	}

	return VersionFrom(info)
}

func VersionFrom(info *debug.BuildInfo) (*BinaryVersion, error) {
	var modified bool
	var revision, buildtime string

	for _, n := range info.Settings {
		switch n.Key {
		case "vcs.revision":
			revision = n.Value
		case "vcs.time":
			buildtime = n.Value
		case "vcs.modified":
			modified = n.Value == "true"
		}
	}

	if revision == "" {
		return nil, fnerrors.InternalError("binary does not include version information")
	}

	v := &BinaryVersion{
		Version:      revision,
		BuildTimeStr: buildtime,
		Modified:     modified,
	}

	if v.BuildTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, v.BuildTimeStr); err == nil {
			v.BuildTime = &parsed
		}
	}

	return v, nil
}
