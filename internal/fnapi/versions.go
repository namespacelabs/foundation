// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"time"
)

type NSRequirements struct {
	MinimumApi int32 `json:"minimum_api"`
}

type GetLatestResponse struct {
	Version   string      `json:"version"`
	BuildTime time.Time   `json:"build_time"`
	Tarballs  []*Artifact `json:"tarballs"`
}

type Artifact struct {
	URL    string `json:"url"`
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	SHA256 string `json:"sha256"`
}

func GetLatestVersion(ctx context.Context, req map[string]any) (*GetLatestResponse, error) {
	var resp GetLatestResponse
	if err := AnonymousCall(ctx, ResolveGlobalEndpoint, "nsl.versions.VersionsService/GetLatest", req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	return &resp, nil
}
