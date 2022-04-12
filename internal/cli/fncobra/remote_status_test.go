// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cuelang.org/go/pkg/time"
	"gotest.tools/assert"
)

func TestFetchLatestRemoteStatusFull(t *testing.T) {
	svr := testServer(t, "{\"tag_name\": \"v0.0.21\", \"created_at\": \"2022-03-31T23:21:43Z\", \"message\": \"test\"}")
	defer svr.Close()

	status, err := FetchLatestRemoteStatus(context.Background(), svr.URL, "myversion")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, status.Message, "test")
	assert.Equal(t, status.TagName, "v0.0.21")
	assert.Equal(t, status.BuildTime.UTC().Format(time.UnixDate), "Thu Mar 31 23:21:43 UTC 2022")
}

func TestFetchLatestRemoteStatusNoMessage(t *testing.T) {
	svr := testServer(t, "{\"tag_name\": \"v0.0.21\", \"created_at\": \"2022-03-31T23:21:43Z\"}")
	defer svr.Close()

	status, err := FetchLatestRemoteStatus(context.Background(), svr.URL, "myversion")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, status.Message, "")
}

func testServer(t *testing.T, response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.DeepEqual(t, r.URL.Query()["current_version"], []string{"myversion"})
		fmt.Fprint(w, response)
	}))
}
