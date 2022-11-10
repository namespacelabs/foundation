// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"time"

	"namespacelabs.dev/foundation/schema"
)

type GetLatestRequest struct {
	NS NSRequirements `json:"ns"`
}

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

func GetLatestVersion(ctx context.Context, nsReqs *schema.Workspace_FoundationRequirements) (*GetLatestResponse, error) {
	// "ns" must always be set.
	req := GetLatestRequest{}
	if nsReqs != nil {
		req.NS = NSRequirements{
			MinimumApi: nsReqs.MinimumApi,
		}
	}

	var resp GetLatestResponse
	if err := (Call[any]{
		Endpoint:     EndpointAddress,
		Method:       "nsl.versions.VersionsService/GetLatest",
		OptionalAuth: true,
	}).Do(ctx, req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	return &resp, nil
}

type GetLatestPrebuiltsRequest struct {
	PackageName []string `json:"package_name,omitempty"`
}

type GetLatestPrebuiltsResponse struct {
	Prebuilt []*GetLatestPrebuiltsResponse_Prebuilt `json:"prebuilt,omitempty"`
}

type GetLatestPrebuiltsResponse_Prebuilt struct {
	PackageName string `json:"package_name,omitempty"`
	Repository  string `json:"repository,omitempty"`
	Digest      string `json:"digest,omitempty"`
}

func GetLatestPrebuilts(ctx context.Context, pkgs ...schema.PackageName) (*GetLatestPrebuiltsResponse, error) {
	req := GetLatestPrebuiltsRequest{
		PackageName: schema.Strs(pkgs...),
	}

	var resp GetLatestPrebuiltsResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.versions.VersionsService/GetLatestPrebuilts", &req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	return &resp, nil
}
