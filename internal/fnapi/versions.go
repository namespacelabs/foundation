// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"
	"time"

	"namespacelabs.dev/foundation/schema"
)

type GetLatestRequest struct {
	TelemetryEnabled bool           `json:"telemetry_enabled"`
	NS               NSRequirements `json:"ns"`
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
	tel := NewTelemetry()
	tel.Enable()
	telemetryEnabled := tel.IsTelemetryEnabled()
	req := GetLatestRequest{
		TelemetryEnabled: telemetryEnabled,
	}
	if nsReqs != nil {
		req.NS = NSRequirements{
			MinimumApi: nsReqs.MinimumApi,
		}
	}
	var resp GetLatestResponse
	if err := callProdAPI(ctx, "nsl.versions.VersionsService/GetLatest", &req, func(dec *json.Decoder) error {
		return dec.Decode(&resp)
	}); err != nil {
		return nil, err
	}

	return &resp, nil
}
