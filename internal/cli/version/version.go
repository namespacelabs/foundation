// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package version

import (
	"os"
	"runtime/debug"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/storage"
)

const DevelopmentBuildVersion = "dev"

// Set by goreleaser to the tag being released.
var Tag = DevelopmentBuildVersion

func Current() (*storage.NamespaceBinaryVersion, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, fnerrors.InternalError("buildinfo is missing")
	}

	return VersionFrom(info)
}

func VersionFrom(info *debug.BuildInfo) (*storage.NamespaceBinaryVersion, error) {
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

	v := &storage.NamespaceBinaryVersion{
		Version:      Tag,
		GitCommit:    revision,
		BuildTimeStr: buildtime,
		Modified:     modified,
	}

	if v.BuildTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, v.BuildTimeStr); err == nil {
			v.BuildTime = timestamppb.New(parsed)
		}
	}

	return v, nil
}

func isDevelopmentBuild(ver *storage.NamespaceBinaryVersion) bool {
	return ver.BuildTime == nil || ver.Version == DevelopmentBuildVersion
}

func ShouldCheckUpdate(ver *storage.NamespaceBinaryVersion) bool {
	if isDevelopmentBuild(ver) {
		return false
	}

	_, have := os.LookupEnv("NS_DO_NOT_UPDATE")
	return !have
}
