// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
)

type StringMatcher struct {
	Values []string               `json:"values,omitempty"`
	Op     StringMatcher_Operator `json:"op,omitempty"`
}

type StringMatcher_Operator int32

const (
	StringMatcher_OPERATOR_UNKNOWN StringMatcher_Operator = 0
	StringMatcher_IS_ANY_OF        StringMatcher_Operator = 1
	StringMatcher_IS_NOT           StringMatcher_Operator = 2
)

type ListBaseImagesRequest struct {
	Filter *ListBaseImagesRequest_Filter `json:"filter,omitempty"`
}

type ListBaseImagesRequest_Filter struct {
	OsLabels *StringMatcher `json:"os_labels,omitempty"`
}

type ListBaseImagesResponse struct {
	Images []*ListBaseImagesResponse_RunnerBaseImage `json:"images,omitempty"`
}

type ListBaseImagesResponse_RunnerBaseImage struct {
	BaseImage   *BaseImage              `json:"base_image,omitempty"`
	Description *RunnerImageDescription `json:"description,omitempty"`
	ImageRef    string                  `json:"image_ref,omitempty"`
}

type BaseImage struct {
	// Base Image unique identifier.
	Id string `json:"id,omitempty"`
	// Deprecated. Use id instead.
	Ref string `json:"ref,omitempty"`
	// Corresponds to InstanceShape.os ("linux" or "macos").
	Os string `json:"os,omitempty"`
	// E.g. Sonoma, Ubuntu LTS
	OsName string `json:"os_name,omitempty"`
	// E.g. 14.4.1
	OsVersion string `json:"os_version,omitempty"`
	// A description used for administration purposes.
	Description string `json:"description,omitempty"`
	// Set of features that the image supports.
	Features []string `json:"features,omitempty"`
}

type RunnerImageDescription struct {
	// OS label as used by GitHub jobs. (e.g. "ubuntu-22.04" or "ubuntu-22.04-staging")
	Label string `json:"label,omitempty"`
	// Image purpose, e.g. production, staging
	Purpose string `json:"purpose,omitempty"`
	// Base image ID
	BaseImageId string `json:"base_image_id,omitempty"`
}

func ListBaseImages(ctx context.Context, osLabel string) (ListBaseImagesResponse, error) {
	var res ListBaseImagesResponse
	if err := (Call[ListBaseImagesRequest]{
		Method:           "nsl.githubrunner.GitHubRunnerService/ListBaseImages",
		IssueBearerToken: IssueBearerToken,
		Retryable:        true,
	}).Do(ctx, ListBaseImagesRequest{
		Filter: &ListBaseImagesRequest_Filter{
			OsLabels: &StringMatcher{
				Values: []string{osLabel},
				Op:     StringMatcher_IS_ANY_OF,
			},
		},
	}, ResolveGlobalEndpoint, DecodeJSONResponse(&res)); err != nil {
		return res, err
	}

	return res, nil
}
