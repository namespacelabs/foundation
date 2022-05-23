// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	uploadPath   = "/bundles/upload"
	downloadPath = "/bundles/download"
)

func init() {
	viper.SetDefault("bundle_api_endpoint", "http://bundles.dev.nslocal.host:40080")
}

type UploadBundleResponse struct {
	BundleId string `json:"bundle_id"`
}

func UploadBundle(ctx context.Context, bundle io.ReadCloser, handle func(res *UploadBundleResponse) error) error {
	endpoint := viper.GetString("bundle_api_endpoint") + uploadPath
	return tasks.Action("fnapi.call").LogLevel(2).Arg("endpoint", endpoint).Run(ctx, func(ctx context.Context) error {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bundle)
		if err != nil {
			return fnerrors.InvocationError("failed to create the upload bundle request: %w", err)
		}

		c := &http.Client{}

		response, err := c.Do(httpReq)
		if err != nil {
			return fnerrors.InvocationError("failed to upload bundle: %w", err)
		}
		defer response.Body.Close()

		dec := json.NewDecoder(response.Body)

		if response.StatusCode == http.StatusOK {
			uploadRes := &UploadBundleResponse{}
			err := dec.Decode(uploadRes)
			if err != nil {
				return fnerrors.InvocationError("failed to decode the upload bundle response: %w", err)
			}
			return handle(uploadRes)
		}

		return fnerrors.InvocationError("failed to upload bundle with status %s", response.Status)
	})
}

type downloadRequest struct {
	BundleId string `json:"bundle_id"`
}

func DownloadBundle(ctx context.Context, bundleId string, handle func(body io.ReadCloser) error) error {
	endpoint := viper.GetString("bundle_api_endpoint") + downloadPath
	return tasks.Action("fnapi.call").LogLevel(2).Arg("endpoint", endpoint).Run(ctx, func(ctx context.Context) error {
		req := &downloadRequest{bundleId}
		reqBytes, err := json.Marshal(req)
		if err != nil {
			return fnerrors.BadInputError("failed to marshal the download bundle request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBytes))
		if err != nil {
			return fnerrors.BadInputError("failed to create the download bundle http request: %w", err)
		}

		c := &http.Client{}
		response, err := c.Do(httpReq)
		if err != nil {
			return fnerrors.InvocationError("failed to download bundle: %w", err)
		}

		switch response.StatusCode {
		case http.StatusOK:
			return handle(response.Body)
		default:
			_ = response.Body.Close()
			return fnerrors.InvocationError("failed to download bundle with status %s", response.Status)
		}
	})
}
