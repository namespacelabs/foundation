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
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

// TODO #93 compute this when we can move the service definition into the foundation repo.
const telemetryServiceName = "telemetry.TelemetryService"
const postTimeout = 1 * time.Second

type Telemetry struct {
	UseTelemetry bool
	errorLogging bool // For testing and debugging.

	backendAddress string
	mu             sync.Mutex // Protects `recID`.
	recID          string     // Set after an invocation is recorded.
}

func NewTelemetry() *Telemetry {
	return &Telemetry{
		UseTelemetry:   true,
		errorLogging:   false,
		backendAddress: "https://grpc-gateway-g793omo8v6okrjjo0v60.prod.namespacelabs.nscloud.dev",
	}
}

func (tel *Telemetry) isTelemetryEnabled() bool {
	doNotTrack := os.Getenv("DO_NOT_TRACK")
	enableTelemetry := viper.GetBool("enable_telemetry")
	ci := os.Getenv("CI")
	return tel.UseTelemetry && doNotTrack == "" && enableTelemetry && ci == ""
}

func (tel *Telemetry) logError(ctx context.Context, err error) {
	if tel.errorLogging {
		fnerrors.Format(console.Stderr(ctx), true, err)
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

type recordInvocationRequest struct {
	Id      string `json:"id,omitempty"`
	Command string `json:"command,omitempty"`
	Arg     []arg  `json:"arg"`
	Flag    []flag `json:"flag"`
	UserId  string `json:"user_id,omitempty"`
	Os      string `json:"os,omitempty"`
	Arch    string `json:"arch,omitempty"`
	NumCpu  int    `json:"num_cpu"`
}

type recordErrorRequest struct {
	Id      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

func readOrGenerateRandID(ctx context.Context, name string) string {
	id := viper.GetString(name)
	if id != "" {
		return id
	}

	id = ids.NewRandomBase62ID(16)

	viper.Set(name, id)
	if err := viper.WriteConfig(); err != nil {
		fmt.Fprintln(console.Warnings(ctx), "failed to persist user-id", err)
	}

	return id
}

// Extracts command name and set flags from cmd. Reports args and flag values in hashed form.
func buildRecordInvocationRequest(ctx context.Context, cmd *cobra.Command, reqID string, args []string) *recordInvocationRequest {
	salt := readOrGenerateRandID(ctx, "telemetry_salt")
	userId := readOrGenerateRandID(ctx, "telemetry_user_id")

	req := recordInvocationRequest{
		Id:      reqID,
		Command: cmd.Use,
		UserId:  userId,
		Os:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		NumCpu:  runtime.NumCPU(),
	}

	cmd.Flags().Visit(func(pflag *pflag.Flag) {
		req.Flag = append(req.Flag, flag{
			Name: pflag.Name,
			Hash: combinedHash(pflag.Value.String(), pflag.Name, salt),
		})
	})

	for _, a := range args {
		req.Arg = append(req.Arg, arg{Hash: combinedHash(a, salt)})
	}

	return &req
}

func (tel *Telemetry) postRecordInvocationRequest(ctx context.Context, req *recordInvocationRequest) error {
	ctx, cancel := context.WithTimeout(ctx, postTimeout)
	defer cancel()

	return callAPI(ctx, tel.backendAddress, fmt.Sprintf("%s/RecordInvocation", telemetryServiceName), req, func(dec *json.Decoder) error {
		return nil // ignore the response
	})
}

func (tel *Telemetry) recordInvocation(ctx context.Context, cmd *cobra.Command, reqID string, args []string) {
	if !tel.isTelemetryEnabled() {
		return
	}

	if viper.GetString("telemetry_user_id") == "" {
		// First fn invocation with Telemetry. Add hint about early access plain text logging.
		// TODO remove before public release.
		out := console.TypedOutput(ctx, "telemetry", tasks.CatOutputUs)
		fmt.Fprint(out, "During the early access, errors are uploaded to our servers for debugging purposes (this will change on our public release).\n")
		fmt.Fprint(out, "This helps us understand what issues you may be hitting.\n")
		fmt.Fprintf(out, "If you'd like to disable this behavior, set %s or %s at %s.\n",
			colors.Bold("DO_NOT_TRACK=1"), colors.Bold("\"enable_telemetry\": false"), viper.ConfigFileUsed())
	}

	req := buildRecordInvocationRequest(ctx, cmd, reqID, args)

	if err := tel.postRecordInvocationRequest(ctx, req); err != nil {
		tel.logError(ctx, err)
		return
	}

	tel.mu.Lock()
	tel.recID = req.Id
	tel.mu.Unlock()
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

	return callAPI(ctx, tel.backendAddress, fmt.Sprintf("%s/RecordError", telemetryServiceName), req, func(dec *json.Decoder) error {
		return nil // ignore the response
	})
}

func (tel *Telemetry) RecordError(ctx context.Context, err error) {
	if !tel.isTelemetryEnabled() || err == nil {
		return
	}

	tel.mu.Lock()
	recID := tel.recID
	tel.mu.Unlock()

	tel.recordError(ctx, recID, err)
}

func (tel *Telemetry) recordError(ctx context.Context, recID string, err error) {
	// If we never saw a recorded ID, bail out.
	if recID == "" {
		tel.logError(ctx, fmt.Errorf("didn't receive telemetry record id"))
		return
	}

	req := recordErrorRequest{Id: recID}

	// TODO remove plain text logging after early access.
	req.Message = err.Error()

	if err := tel.postRecordErrorRequest(ctx, req); err != nil {
		tel.logError(ctx, err)
	}
}
