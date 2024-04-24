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
	res, err := w.rt.RoundTrip(req)
	if !strings.HasSuffix(req.URL.Path, "/secret-id") ||
		err != nil ||
		res == nil ||
		res.StatusCode != http.StatusOK ||
		strings.Split(res.Header.Get("content-type"), ";")[0] != "application/json" {
		return res, err
	}
	defer res.Body.Close()

	body := map[string]any{}
	json.NewDecoder(res.Body).Decode(&body)

	if data, ok := body["data"].(map[string]any); ok {
		if ttl, ok := data["secret_id_ttl"].(float64); ok {
			data["secret_id_ttl"] = strconv.Itoa(int(ttl))
		}
	}

	raw, _ := json.Marshal(body)
	res.Body = io.NopCloser(bytes.NewReader(raw))

	return res, err
}
