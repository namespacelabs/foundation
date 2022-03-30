// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gotest.tools/assert"
)

func TestTelemetryDisabled(t *testing.T) {
	reset := setupEnv(t)
	defer reset()

	tel := &Telemetry{
		UseTelemetry: false,
		errorLogging: true,
		makeClientID: generateTestIDs,
	}

	cmd := &cobra.Command{
		Use: "fake-command",
		Run: func(cmd *cobra.Command, args []string) {
			tel.RecordInvocation(context.Background(), cmd, args)
		}}

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("Calls to TelemetryService are fobidden when telemetry is disabled.")
	}))
	defer svr.Close()

	tel.backendAddress = svr.URL

	cmd.Execute()
	tel.RecordError(context.Background(), fmt.Errorf("foo error"))
}

func generateTestIDs(ctx context.Context) (clientID, bool) {
	return clientID{newRandID(), newRandID()}, false
}

func TestTelemetryDisabledViaEnv(t *testing.T) {
	reset := setupEnv(t)
	defer reset()

	tel := &Telemetry{
		UseTelemetry: true,
		errorLogging: true,
		makeClientID: generateTestIDs,
	}
	t.Setenv("DO_NOT_TRACK", "1")

	cmd := &cobra.Command{
		Use: "fake-command",
		Run: func(cmd *cobra.Command, args []string) {
			tel.RecordInvocation(context.Background(), cmd, args)
		}}

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.EscapedPath(), "/telemetry.TelemetryService") {
			t.Errorf("Calls to TelemetryService are fobidden when telemetry is disabled.")
		}

	}))
	defer svr.Close()

	tel.backendAddress = svr.URL

	cmd.Execute()
	tel.RecordError(context.Background(), fmt.Errorf("foo error"))
}

func TestTelemetryDisabledViaViper(t *testing.T) {
	reset := setupEnv(t)
	defer reset()

	viper.Set("enable_telemetry", false)

	tel := &Telemetry{
		UseTelemetry: true,
		errorLogging: true,
		makeClientID: generateTestIDs,
	}

	cmd := &cobra.Command{
		Use: "fake-command",
		Run: func(cmd *cobra.Command, args []string) {
			tel.RecordInvocation(context.Background(), cmd, args)
		}}

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.EscapedPath(), "/telemetry.TelemetryService") {
			t.Errorf("Calls to TelemetryService are fobidden when telemetry is disabled.")
		}

	}))
	defer svr.Close()

	tel.backendAddress = svr.URL

	cmd.Execute()
	tel.RecordError(context.Background(), fmt.Errorf("foo error"))
}

func TestTelemetryRecordInvocationAnon(t *testing.T) {
	reset := setupEnv(t)
	defer reset()

	tel := &Telemetry{
		UseTelemetry: true,
		errorLogging: true,
		makeClientID: generateTestIDs,
	}

	sentID := make(chan string, 1)
	cmd := &cobra.Command{
		Use: "fake-command",
		Run: func(cmd *cobra.Command, args []string) {
			defer close(sentID)
			sentID <- tel.RecordInvocation(context.Background(), cmd, args)
		}}
	cmd.PersistentFlags().Bool("dummy_flag", false, "")

	fakeArg := "fake/arg/value"
	fakeArgs := []string{"--dummy_flag", fakeArg}
	cmd.SetArgs(fakeArgs)

	var req recordInvocationRequest

	receivedID := make(chan string, 1)
	svr := httptest.NewServer(assertGrpcInvocation(t, "/telemetry.TelemetryService/RecordInvocation", &req, func(w http.ResponseWriter) {
		defer close(receivedID)

		assert.Equal(t, req.Command, cmd.Use, req)

		// Assert that we don't transmit user data in plain text.
		assert.Equal(t, len(req.Arg), 1, req)
		assert.Assert(t, req.Arg[0].Hash != fakeArg, req)
		assert.Equal(t, req.Arg[0].Plaintext, "", req)
		assert.Equal(t, len(req.Flag), 1, req)
		assert.Equal(t, req.Flag[0].Name, "dummy_flag", req)
		assert.Assert(t, req.Flag[0].Hash != "true", req)
		assert.Equal(t, req.Flag[0].Plaintext, "", req)

		receivedID <- req.ID
	}))

	defer svr.Close()

	tel.backendAddress = svr.URL

	err := cmd.Execute()
	assert.NilError(t, err)

	assert.Equal(t, <-receivedID, <-sentID) // Make sure we validated the request.
}

func TestTelemetryRecordErrorPlaintext(t *testing.T) {
	reset := setupEnv(t)
	defer reset()

	tel := &Telemetry{
		UseTelemetry: true,
		errorLogging: true,
		recID:        "fake-id",
		makeClientID: generateTestIDs,
	}

	var req recordErrorRequest
	receivedID := make(chan string, 1)
	svr := httptest.NewServer(assertGrpcInvocation(t, "/telemetry.TelemetryService/RecordError", &req, func(_ http.ResponseWriter) {
		defer close(receivedID)

		assert.Assert(t, req.Message != "", req)

		receivedID <- req.ID
	}))
	defer svr.Close()

	tel.backendAddress = svr.URL

	tel.RecordError(context.Background(), fmt.Errorf("foo error"))

	// Assert on intercepted request outside the HandlerFunc to ensure the handler is called
	assert.Equal(t, <-receivedID, tel.recID)
}

func assertGrpcInvocation(t *testing.T, method string, request interface{}, handle func(http.ResponseWriter)) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method != "POST" {
			t.Errorf("expected method=POST, got method=%v", r.Method)
		}

		bodyBytes, err := io.ReadAll(r.Body)
		assert.NilError(t, err)

		if r.URL.EscapedPath() == method {
			err := json.Unmarshal(bodyBytes, request)
			assert.NilError(t, err)
			handle(rw)
		} else {
			t.Errorf("expected url=%q, got url=%q", method, r.URL.EscapedPath())
		}
	}
}

func setupEnv(t *testing.T) func() {
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("CI", "")

	viper.Set("enable_telemetry", true)

	return func() { viper.Reset() }
}
