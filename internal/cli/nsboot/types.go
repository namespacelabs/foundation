// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nsboot

import "time"

// Represents a tool version reference as reported by the version-check endpoint.
type toolVersion struct {
	TagName   string    `json:"tag_name"`
	BuildTime time.Time `json:"build_time"`
	FetchedAt time.Time `json:"fetched_at"`
	URL       string    `json:"tarball_url"`
	SHA256    string    `json:"tarball_sha256"`
}

type reportedExistingVersion struct {
	TagName string `json:"tag_name"`
	SHA256  string `json:"sha256"`
}

// Schema for $CACHE/tool/versions.json.
type versionCache struct {
	Latest     *toolVersion `json:"latest"`
	BinaryPath string       `json:"binary_path"` // The path to a previously resolved binary.
}
