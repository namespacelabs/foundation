// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/go-ids"
)

// TODO #93 compute this when we can move the service definition into the foundation repo.
const telemetryServiceName = "telemetry.TelemetryService"
const postTimeout = 1 * time.Second

type Telemetry struct {
	UseTelemetry bool
	errorLogging bool // For testing and debugging.

	backendAddress string
	recID          atomic.String // Set after an invocation is recorded.
	// The client ID should be anonymous and never be associated with any user credentials.
	makeClientID func(context.Context) (clientID, bool)
}

func NewTelemetry() *Telemetry {
	return &Telemetry{
		UseTelemetry:   true,
		errorLogging:   false,
		backendAddress: viper.GetString("api_endpoint"),
		makeClientID:   generateClientIDAndSalt,
	}
}

func (tel *Telemetry) IsTelemetryEnabled() bool {
	doNotTrack := os.Getenv("DO_NOT_TRACK")
	enableTelemetry := viper.GetBool("enable_telemetry")
	return !environment.IsRunningInCI() && tel.UseTelemetry && doNotTrack == "" && enableTelemetry
}

func (tel *Telemetry) logError(ctx context.Context, err error) {
	if tel.errorLogging {
		fnerrors.Format(console.Stderr(ctx), err)
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

type clientID struct {
	ID   string `json:"id"`
	Salt string `json:"salt"`
}

func newRandID() string {
	return ids.NewRandomBase62ID(16)
}

func generateClientIDAndSalt(ctx context.Context) (clientID, bool) {
	configDir, err := dirs.Config()
	if err != nil {
		return clientID{newRandID(), newRandID()}, false
	}

	idfile := filepath.Join(configDir, "clientid.json")
	idcontents, err := ioutil.ReadFile(idfile)
	if err == nil {
		var clientID clientID
		if err := json.Unmarshal(idcontents, &clientID); err == nil {
			if clientID.ID != "" && clientID.Salt != "" {
				return clientID, false
			}
		}
	}

	newClientID := clientID{newRandID(), newRandID()}
	if err := writeJSON(idfile, newClientID); err != nil {
		fmt.Fprintln(console.Warnings(ctx), "failed to persist user-id", err)
	}

	return newClientID, true
}

func writeJSON(path string, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0644)
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
func buildRecordInvocationRequest(ctx context.Context, cmd *cobra.Command, c clientID, reqID string, args []string) *recordInvocationRequest {
	req := recordInvocationRequest{
		ID:      reqID,
		Command: fullCommand(cmd),
		UserId:  c.ID,
		Os:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		NumCpu:  runtime.NumCPU(),
	}

	if v, err := version.Version(); err == nil {
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

	return CallAPI(ctx, tel.backendAddress, fmt.Sprintf("%s/RecordInvocation", telemetryServiceName), req, func(dec *json.Decoder) error {
		return nil // ignore the response
	})
}

func (tel *Telemetry) recordInvocation(ctx context.Context, cmd *cobra.Command, reqID string, args []string) {
	if !tel.IsTelemetryEnabled() {
		return
	}

	c, _ := tel.makeClientID(ctx)

	req := buildRecordInvocationRequest(ctx, cmd, c, reqID, args)

	tel.recID.Store(req.ID)

	if err := tel.postRecordInvocationRequest(ctx, req); err != nil {
		tel.logError(ctx, err)
		return
	}
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

	return CallAPI(ctx, tel.backendAddress, fmt.Sprintf("%s/RecordError", telemetryServiceName), req, func(dec *json.Decoder) error {
		return nil // ignore the response
	})
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
