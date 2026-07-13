// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	oldproto "github.com/golang/protobuf/proto"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
	"namespacelabs.dev/integrations/nsc/grpcapi"
)

const streamBuildLogsMethod = "/nsl.vm.builds.GlobalBuildsService/StreamBuildLogs"

// getBuildLogsRequest mirrors the request wire type for StreamBuildLogs.
type getBuildLogsRequest struct {
	BuildRef string `protobuf:"bytes,1,opt,name=build_ref,json=buildRef,proto3" json:"build_ref,omitempty"`
}

func (r *getBuildLogsRequest) Reset()         { *r = getBuildLogsRequest{} }
func (r *getBuildLogsRequest) String() string { return oldproto.CompactTextString(r) }
func (*getBuildLogsRequest) ProtoMessage()    {}

func newBuildLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <build-ref>",
		Short: "Print logs for a build.",
		Args:  cobra.ExactArgs(1),
	}

	output := cmd.Flags().StringP("output", "o", "plain", "Output format. Supported values: plain, json (BuildKit raw JSON, one object per line).")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *output != "plain" && *output != "json" {
			return fnerrors.BadInputError("unsupported output format %q, supported values: plain, json", *output)
		}

		recv, closeStream, err := openBuildLogsStream(ctx, args[0])
		if err != nil {
			return err
		}
		defer closeStream()

		return writeBuildLogs(ctx, console.Stdout(ctx), *output, recv)
	})

	return cmd
}

func openBuildLogsStream(ctx context.Context, buildRef string) (func() (string, error), func() error, error) {
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, nil, err
	}

	resolved, err := fnapi.IssueBearerTokenFromToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	apiEndpoint, err := endpoint.ResolveRegionalEndpoint(ctx, resolved)
	if err != nil {
		return nil, nil, err
	}

	conn, err := grpcapi.NewConnectionWithEndpoint(ctx, apiEndpoint, token)
	if err != nil {
		return nil, nil, err
	}

	desc := &grpc.StreamDesc{ServerStreams: true}
	stream, err := grpc.NewClientStream(ctx, desc, conn, streamBuildLogsMethod)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	if err := stream.SendMsg(&getBuildLogsRequest{BuildRef: buildRef}); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if err := stream.CloseSend(); err != nil {
		conn.Close()
		return nil, nil, err
	}

	recv := func() (string, error) {
		msg := &wrapperspb.StringValue{}
		if err := stream.RecvMsg(msg); err != nil {
			return "", err
		}
		return msg.Value, nil
	}

	return recv, conn.Close, nil
}

func writeBuildLogs(ctx context.Context, out io.Writer, output string, recv func() (string, error)) error {
	var mode progressui.DisplayMode
	switch output {
	case "json":
		mode = progressui.RawJSONMode
	case "plain":
		mode = progressui.PlainMode
	default:
		return fnerrors.BadInputError("unsupported output format %q, supported values: plain, json", output)
	}

	display, err := progressui.NewDisplay(out, mode)
	if err != nil {
		return err
	}

	statusCh := make(chan *client.SolveStatus)
	recvErr := make(chan error, 1)
	go func() {
		defer close(statusCh)
		received := false
		for {
			value, err := recv()
			if isBuildLogsEOF(err, received) {
				recvErr <- nil
				return
			}
			if err != nil {
				recvErr <- err
				return
			}
			received = true

			var status controlapi.StatusResponse
			if err := json.Unmarshal([]byte(value), &status); err != nil {
				recvErr <- fmt.Errorf("failed to decode build log entry: %w", err)
				return
			}

			select {
			case statusCh <- client.NewSolveStatus(&status):
			case <-ctx.Done():
				recvErr <- context.Cause(ctx)
				return
			}
		}
	}()

	_, displayErr := display.UpdateFrom(ctx, statusCh)
	if err := <-recvErr; err != nil {
		return err
	}
	return displayErr
}

func isBuildLogsEOF(err error, received bool) bool {
	if err == io.EOF {
		return true
	}

	return received && status.Code(err) == codes.Internal && status.Convert(err).Message() == "server closed the stream without sending trailers"
}
