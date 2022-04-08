// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"
	"errors"
	"flag"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/logoutput"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/provision/tool/grpcstdio"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/schema"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

type Request struct {
	Snapshots map[string]fs.FS
	r         *protocol.ToolRequest
}

type StackRequest struct {
	Request
	Env   *schema.Environment
	Focus *schema.Stack_Entry
	Stack *schema.Stack
}

type MakeExtension interface {
	ToDefinition() (*schema.DefExtension, error)
}

type ApplyOutput struct {
	Definitions []defs.MakeDefinition
	Extensions  []MakeExtension
}

type DeleteOutput struct {
	Ops []defs.MakeDefinition
}

type StackHandler interface {
	Apply(context.Context, StackRequest, *ApplyOutput) error
	Delete(context.Context, StackRequest, *DeleteOutput) error
}

// XXX remove Tool when all the uses are gone.
type Tool interface {
	StackHandler
}

type AllHandlers interface {
	StackHandler

	Invoke(context.Context, Request) (*protocol.InvokeResponse, error)
}

func (p Request) UnpackInput(msg proto.Message) error {
	if msg == nil {
		return errors.New("msg is nil")
	}

	for _, env := range p.r.Input {
		if env.MessageIs(msg) {
			return env.UnmarshalTo(msg)
		}
	}

	return fnerrors.InternalError("no such env: %s", msg.ProtoReflect().Descriptor().FullName())
}

// PackageOwner returns the name of the package that defined this tool.
func (p Request) PackageOwner() string {
	return p.r.GetToolPackage()
}

func RunTool(t Tool) {
	run(context.Background(), handlerCompat{t})
}

func run(ctx context.Context, h AllHandlers) {
	flag.Parse()

	ctx = logoutput.WithOutput(ctx, logoutput.OutputTo{Writer: os.Stderr})

	if err := handle(ctx, h); err != nil {
		log.Fatal(err)
	}
}

func handle(ctx context.Context, h AllHandlers) error {
	conn, err := grpc.DialContext(ctx, "stdio",
		grpc.WithInsecure(),
		grpc.WithReadBufferSize(0),
		grpc.WithWriteBufferSize(0),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return grpcstdio.NewConnection(os.Stdout, os.Stdin), nil
		}))
	if err != nil {
		return err
	}

	defer conn.Close()

	cli := protocol.NewInvocationServiceClient(conn)
	stream, err := cli.Worker(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&protocol.WorkerChunk{ClientHello: &protocol.WorkerChunk_ClientHello{
		FnApiVersion:   versions.APIVersion,
		ToolApiVersion: versions.ToolAPIVersion,
	}}); err != nil {
		return err
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		if msg.ToolRequest != nil {
			response, err := handleRequest(ctx, msg.ToolRequest, h)
			if err != nil {
				return err
			}

			if err := stream.Send(&protocol.WorkerChunk{ToolResponse: response}); err != nil {
				return err
			}

			if err := stream.CloseSend(); err != nil {
				return err
			}

			// Make sure that the send was received.
			if _, err := stream.Recv(); err != nil {
				if err != io.EOF {
					return err
				}
			}

			return nil
		}
	}
}

type handlerCompat struct {
	tool Tool
}

var _ AllHandlers = handlerCompat{}

func (h handlerCompat) Apply(ctx context.Context, req StackRequest, output *ApplyOutput) error {
	return h.tool.Apply(ctx, req, output)
}

func (h handlerCompat) Delete(ctx context.Context, req StackRequest, output *DeleteOutput) error {
	return h.tool.Delete(ctx, req, output)
}

func (h handlerCompat) Invoke(context.Context, Request) (*protocol.InvokeResponse, error) {
	return nil, status.Error(codes.Unavailable, "invoke not supported")
}

func Handle(h *Handlers) {
	run(context.Background(), runHandlers{h})
}

func HandleInvoke(f InvokeFunc) {
	h := NewHandlers()
	h.Any().HandleInvoke(f)
	Handle(h)
}

func runTool(ctx context.Context, r io.Reader, w io.Writer, t AllHandlers) error {
	reqBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	req := &protocol.ToolRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return err
	}

	response, err := handleRequest(ctx, req, t)
	if err != nil {
		return err
	}

	serialized, err := proto.Marshal(response)
	if err != nil {
		return err
	}

	w.Write(serialized)
	return nil
}

func handleRequest(ctx context.Context, req *protocol.ToolRequest, t AllHandlers) (*protocol.ToolResponse, error) {
	var br Request
	br.r = req
	br.Snapshots = map[string]fs.FS{}

	for _, snapshot := range req.Snapshot {
		var m memfs.FS
		for _, entry := range snapshot.Entry {
			m.Add(entry.Path, entry.Contents)
		}
		br.Snapshots[snapshot.Name] = &m
	}

	response := &protocol.ToolResponse{}

	switch x := req.RequestType.(type) {
	case *protocol.ToolRequest_ApplyRequest:
		p, err := parseStackRequest(br, x.ApplyRequest.Header)
		if err != nil {
			return nil, err
		}

		var out ApplyOutput
		if err := t.Apply(ctx, p, &out); err != nil {
			return nil, err
		}

		response.ApplyResponse = &protocol.ApplyResponse{}
		for _, input := range out.Extensions {
			packed, err := input.ToDefinition()
			if err != nil {
				return nil, err
			}

			if packed.For == "" {
				packed.For = p.Focus.GetPackageName().String()
			}

			response.ApplyResponse.Extension = append(response.ApplyResponse.Extension, packed)
		}

		response.ApplyResponse.Definition, err = defs.Make(out.Definitions...)
		if err != nil {
			return nil, err
		}

	case *protocol.ToolRequest_DeleteRequest:
		p, err := parseStackRequest(br, x.DeleteRequest.Header)
		if err != nil {
			return nil, err
		}

		var out DeleteOutput
		if err := t.Delete(ctx, p, &out); err != nil {
			return nil, err
		}

		response.DeleteResponse = &protocol.DeleteResponse{}
		response.DeleteResponse.Definition, err = defs.Make(out.Ops...)
		if err != nil {
			return nil, err
		}

	case *protocol.ToolRequest_InvokeRequest:
		output, err := t.Invoke(ctx, br)
		if err != nil {
			return nil, err
		}
		response.InvokeResponse = output
	}

	return response, nil
}

func parseStackRequest(br Request, header *protocol.StackRelated) (StackRequest, error) {
	if header == nil {
		// This is temporary, while we move from top-level fields to {Apply,Delete} specific ones.
		header = &protocol.StackRelated{
			FocusedServer: br.r.FocusedServer,
			Env:           br.r.Env,
			Stack:         br.r.Stack,
		}
	}

	var p StackRequest

	s := header.Stack.GetServer(schema.PackageName(header.FocusedServer))
	if s == nil {
		return p, fnerrors.InternalError("%s: focused server not present in the stack", header.FocusedServer)
	}

	p.Request = br
	p.Env = header.Env
	p.Focus = s
	p.Stack = header.Stack

	return p, nil
}
