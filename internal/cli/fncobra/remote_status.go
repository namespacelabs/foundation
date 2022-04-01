// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
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

type Release struct {
	TagName   string
	BuildTime time.Time
}

type RemoteStatus struct {
	LatestRelease Release
	Message       string
}

// Used to get the latest release version and potentially a message for the users.
func FetchLatestRemoteStatus(baseUrl string, currentVer string) (*RemoteStatus, error) {
	fullUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	q := fullUrl.Query()
	q.Set("current_version", currentVer)
	fullUrl.RawQuery = q.Encode()
	response, err := http.Get(fullUrl.String())
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var remoteStatus rawRemoteStatus
	err = json.Unmarshal(body, &remoteStatus)
	if err != nil {
		return nil, err
	}
	latestBuildTime, err := time.Parse(time.RFC3339, remoteStatus.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &RemoteStatus{
		Message: remoteStatus.Message,
		LatestRelease: Release{
			TagName:   remoteStatus.TagName,
			BuildTime: latestBuildTime,
		},
	}, nil
}
