// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type rawRemoteStatus struct {
	TagName   string `json:"tag_name"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

type remoteStatus struct {
	Version    string
	NewVersion bool
	BuildTime  time.Time
	Message    string
}

const versionCheckEndpoint = "https://foundation-version.namespacelabs.workers.dev"

// Used to get the latest release version and potentially a message for the users.
func FetchLatestRemoteStatus(ctx context.Context, baseUrl string, currentVer string) (*remoteStatus, error) {
	fullUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	q := fullUrl.Query()
	q.Set("current_version", currentVer)

	fullUrl.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	c := &http.Client{}
	response, err := c.Do(httpReq)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var rs rawRemoteStatus
	if err := json.Unmarshal(body, &rs); err != nil {
		return nil, err
	}

	latestBuildTime, err := time.Parse(time.RFC3339, rs.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &remoteStatus{
		Message:   rs.Message,
		Version:   rs.TagName,
		BuildTime: latestBuildTime,
	}, nil
}
