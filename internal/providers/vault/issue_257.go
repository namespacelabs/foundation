// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/vault-client-go"
)

// Temporary workaround until the fix is merged:
// https://github.com/hashicorp/vault-client-go/pull/260
func withIssue257Workaround() vault.ClientOption {
	c := vault.DefaultConfiguration().HTTPClient
	c.Transport = fix257{rt: c.Transport}
	return vault.WithHTTPClient(c)
}

type fix257 struct {
	rt http.RoundTripper
}

func (w fix257) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "/secret-id") {
		return w.rt.RoundTrip(req)
	}
	res, err := w.rt.RoundTrip(req)
	if err != nil {
		return res, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK ||
		strings.Split(res.Header.Get("content-type"), ";")[0] != "application/json" {
		return res, err
	}

	contents, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	body := map[string]any{}
	// If JSON decoding fails, don't change the body.
	if err := json.Unmarshal(contents, &body); err == nil {
		if data, ok := body["data"].(map[string]any); ok {
			if ttl, ok := data["secret_id_ttl"].(float64); ok {
				// AppRoleWriteSecretIdResponse.data.secret_id_ttl is of type string.
				data["secret_id_ttl"] = strconv.Itoa(int(ttl))
			}
		}
	}

	// If JSON re-encoding fails, also don't change the body.
	if recoded, err := json.Marshal(body); err == nil {
		contents = recoded
	}
	res.Body = io.NopCloser(bytes.NewReader(contents))

	return res, err
}
