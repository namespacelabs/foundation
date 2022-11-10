// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/atomic"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/go-ids"
)

// TODO #93 compute this when we can move the service definition into the foundation repo.
const telemetryServiceName = "telemetry.TelemetryService"
const postTimeout = 1 * time.Second

type Telemetry struct {
	useTelemetry bool
	errorLogging bool // For testing and debugging.

	backendAddress string
	recID          *atomic.String // Never nil, set after an invocation is recorded.
	id             ephemeralCliID
	created        bool // True if this the first invocation with a new ID.
}

func NewTelemetry(ctx context.Context) *Telemetry {
	return InternalNewTelemetry(ctx, getOrGenerateEphemeralCliID)
}

func InternalNewTelemetry(ctx context.Context, makeID func(context.Context) (ephemeralCliID, bool)) *Telemetry {
	id, created := makeID(ctx)

	return &Telemetry{
		errorLogging:   false,
		backendAddress: EndpointAddress,
		recID:          atomic.NewString(""),
		id:             id,
		created:        created,
	}
}

type contextKey string

var (
	_telemetryKey = contextKey("fn.telemetry")
)

func TelemetryOn(ctx context.Context) *Telemetry {
	v := ctx.Value(_telemetryKey)
	if v == nil {
		return nil
	}
	return v.(*Telemetry)
}

func WithTelemetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, _telemetryKey, NewTelemetry(ctx))
}

// Telemetry needs to be excplicitly enabled by calling this function.
// IsTelemetryEnabled() may still be false if telemetry is disabled through DO_NOT_TRACK, etc.
func (tel *Telemetry) Enable() {
	tel.useTelemetry = true
}

func (tel *Telemetry) IsTelemetryEnabled() bool {
	doNotTrack := os.Getenv("DO_NOT_TRACK")
	enableTelemetry := viper.GetBool("enable_telemetry")
	return !environment.IsRunningInCI() && tel.useTelemetry && doNotTrack == "" && enableTelemetry
}

func (tel *Telemetry) logError(ctx context.Context, err error) {
	if tel.errorLogging {
		format.Format(console.Stderr(ctx), err)
	}
}

func combinedHash(ins ...string) string {
	h := sha256.New()
	for _, in := range ins {
		h.Write([]byte(in))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TODO #93 remove structs when we can move the service definition into the foundation repo.
type flag struct {
	Name      string `json:"name,omitempty"`
	Hash      string `json:"hash,omitempty"`
	Plaintext string `json:"plaintext,omitempty"`
}

type arg struct {
	Hash      string `json:"hash,omitempty"`
	Plaintext string `json:"plaintext,omitempty"`
}

type binaryVersion struct {
	Version   string `json:"version,omitempty"`
	BuildTime string `json:"build_time,omitempty"`
	Modified  bool   `json:"modified,omitempty"`
}

type recordInvocationRequest struct {
	ID      string         `json:"id,omitempty"`
	Command string         `json:"command,omitempty"`
	Arg     []arg          `json:"arg"`
	Flag    []flag         `json:"flag"`
	UserId  string         `json:"user_id,omitempty"`
	Os      string         `json:"os,omitempty"`
	Arch    string         `json:"arch,omitempty"`
	NumCpu  int            `json:"num_cpu"`
	Version *binaryVersion `json:"version"`
}

type recordErrorRequest struct {
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

type ephemeralCliID struct {
	ID   string `json:"id"`
	Salt string `json:"salt"`
}

func newRandID() string {
	return ids.NewRandomBase62ID(16)
}

func getOrGenerateEphemeralCliID(ctx context.Context) (ephemeralCliID, bool) {
	configDir, err := dirs.Config()
	if err != nil {
		panic(err) // XXX Config() should not return an error.
	}

	idfile := filepath.Join(configDir, "clientid.json")
	idcontents, err := os.ReadFile(idfile)
	if err == nil {
		var clientID ephemeralCliID
		if err := json.Unmarshal(idcontents, &clientID); err == nil {
			if clientID.ID != "" && clientID.Salt != "" {
				return clientID, false
			}
		}
	}

	newClientID := ephemeralCliID{newRandID(), newRandID()}
	if err := writeJSON(idfile, newClientID); err != nil {
		fmt.Fprintln(console.Warnings(ctx), "failed to persist user-id", err)
	}

	return newClientID, os.IsNotExist(err)
}

func writeJSON(path string, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func fullCommand(cmd *cobra.Command) string {
	commands := []string{cmd.Use}
	for cmd.HasParent() {
		cmd = cmd.Parent()
		commands = append([]string{cmd.Use}, commands...)
	}
	return strings.Join(commands, " ")
}

// Extracts command name and set flags from cmd. Reports args and flag values in hashed form.
func buildRecordInvocationRequest(ctx context.Context, cmd *cobra.Command, c ephemeralCliID, reqID string, args []string) *recordInvocationRequest {
	req := recordInvocationRequest{
		ID:      reqID,
		Command: fullCommand(cmd),
		UserId:  c.ID,
		Os:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		NumCpu:  runtime.NumCPU(),
	}

	if v, err := version.Current(); err == nil {
		if v.Modified {
			// don't upload version information on modified binaries
			req.Version = &binaryVersion{
				Modified: true,
			}
		} else {
			req.Version = &binaryVersion{
				Version:   v.GitCommit,
				BuildTime: v.BuildTimeStr,
				Modified:  false,
			}
		}
	}

	cmd.Flags().Visit(func(pflag *pflag.Flag) {
		req.Flag = append(req.Flag, flag{
			Name: pflag.Name,
			Hash: combinedHash(pflag.Value.String(), pflag.Name, c.Salt),
		})
	})

	for _, a := range args {
		req.Arg = append(req.Arg, arg{Hash: combinedHash(a, c.Salt)})
	}

	return &req
}

func (tel *Telemetry) postRecordInvocationRequest(ctx context.Context, req *recordInvocationRequest) error {
	ctx, cancel := context.WithTimeout(ctx, postTimeout)
	defer cancel()

	record := Call[recordInvocationRequest]{
		Endpoint:     tel.backendAddress,
		Method:       fmt.Sprintf("%s/RecordInvocation", telemetryServiceName),
		OptionalAuth: true,
	}

	return record.Do(ctx, *req, nil)
}

func (tel *Telemetry) recordInvocation(ctx context.Context, cmd *cobra.Command, reqID string, args []string) {
	if !tel.IsTelemetryEnabled() {
		return
	}

	req := buildRecordInvocationRequest(ctx, cmd, tel.id, reqID, args)

	if err := tel.postRecordInvocationRequest(ctx, req); err != nil {
		tel.logError(ctx, err)
		return
	}

	// Only store request id if recoding the invocation succeeded.
	tel.recID.Store(req.ID)
}

func (tel *Telemetry) RecordInvocation(ctx context.Context, cmd *cobra.Command, args []string) string {
	reqID := ids.NewRandomBase62ID(16)

	// Telemetry should be best effort and not block the user.
	go tel.recordInvocation(ctx, cmd, reqID, args)

	return reqID
}

func (tel *Telemetry) postRecordErrorRequest(ctx context.Context, req recordErrorRequest) error {
	ctx, cancel := context.WithTimeout(ctx, postTimeout)
	defer cancel()

	return AnonymousCall(ctx, tel.backendAddress, fmt.Sprintf("%s/RecordError", telemetryServiceName), req, nil)
}

func (tel *Telemetry) RecordError(ctx context.Context, err error) {
	if !tel.IsTelemetryEnabled() || err == nil {
		return
	}

	tel.recordError(ctx, tel.recID.Load(), err)
}

func (tel *Telemetry) recordError(ctx context.Context, recID string, err error) {
	errStr, isExpected := fnerrors.IsExpected(err)
	if isExpected {
		// We are only interested in unexpected errors.
		return
	}

	// If we never saw a recorded ID, bail out.
	if recID == "" {
		tel.logError(ctx, fmt.Errorf("didn't receive telemetry record id"))
		return
	}

	req := recordErrorRequest{ID: recID}

	// TODO remove plain text logging after early access.
	req.Message = errStr

	if err := tel.postRecordErrorRequest(ctx, req); err != nil {
		tel.logError(ctx, err)
	}
}

func (tel *Telemetry) IsFirstRun() bool { return tel.created }

func (tel *Telemetry) GetClientID() string {
	if tel == nil {
		return ""
	}

	if !tel.IsTelemetryEnabled() {
		return ""
	}

	return tel.id.ID
}
