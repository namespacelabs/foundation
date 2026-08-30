// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	oldproto "github.com/golang/protobuf/proto"
	oldtimestamp "github.com/golang/protobuf/ptypes/timestamp"
	controlapi "github.com/moby/buildkit/api/services/control"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetBuildLogsRequestWireFormat(t *testing.T) {
	got, err := oldproto.Marshal(&getBuildLogsRequest{BuildRef: "ref"})
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0x0a, 0x03, 'r', 'e', 'f'}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected wire encoding: got %x, want %x", got, want)
	}
}

func TestWriteBuildLogsJSON(t *testing.T) {
	serviceRecord, err := json.Marshal(&controlapi.StatusResponse{
		Logs: []*controlapi.VertexLog{{
			Vertex:    "sha256:abc",
			Timestamp: &oldtimestamp.Timestamp{},
			Stream:    1,
			Msg:       []byte("hello\n"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	recv := buildLogReceiver([]string{string(serviceRecord)}, nil)
	var out bytes.Buffer

	if err := writeBuildLogs(context.Background(), &out, "json", recv); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "{\"logs\":[{\"vertex\":\"sha256:abc\",\"stream\":1,\"data\":\"aGVsbG8K\",\"timestamp\":\"1970-01-01T00:00:00Z\"}]}\n"; got != want {
		t.Fatalf("unexpected output: got %q, want %q", got, want)
	}
}

func TestWriteBuildLogsPlain(t *testing.T) {
	recv := buildLogReceiver([]string{"{\"vertexes\":[]}"}, nil)

	if err := writeBuildLogs(context.Background(), io.Discard, "plain", recv); err != nil {
		t.Fatal(err)
	}
}

func TestWriteBuildLogsPropagatesStreamError(t *testing.T) {
	want := errors.New("stream failed")
	recv := buildLogReceiver(nil, want)

	if err := writeBuildLogs(context.Background(), io.Discard, "json", recv); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func TestWriteBuildLogsAcceptsMissingTrailersAfterData(t *testing.T) {
	missingTrailers := status.Error(codes.Internal, "server closed the stream without sending trailers")
	recv := buildLogReceiver([]string{"{\"vertexes\":[]}"}, missingTrailers)
	var out bytes.Buffer

	if err := writeBuildLogs(context.Background(), &out, "json", recv); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "{}\n"; got != want {
		t.Fatalf("unexpected output: got %q, want %q", got, want)
	}
}

func TestWriteBuildLogsRejectsMissingTrailersWithoutData(t *testing.T) {
	missingTrailers := status.Error(codes.Internal, "server closed the stream without sending trailers")
	recv := buildLogReceiver(nil, missingTrailers)

	if err := writeBuildLogs(context.Background(), io.Discard, "json", recv); !errors.Is(err, missingTrailers) {
		t.Fatalf("got %v, want %v", err, missingTrailers)
	}
}

func buildLogReceiver(values []string, finalErr error) func() (string, error) {
	next := 0
	return func() (string, error) {
		if next < len(values) {
			value := values[next]
			next++
			return value, nil
		}
		if finalErr != nil {
			return "", finalErr
		}
		return "", io.EOF
	}
}
