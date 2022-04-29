// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"
	"errors"
	"io/fs"
	"log"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/provision/tool/protocol"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

type Request struct {
	Snapshots map[string]fs.FS
	r         *protocol.ToolRequest
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

	return fnerrors.InternalError("no such input: %s", msg.ProtoReflect().Descriptor().FullName())
}

// PackageOwner returns the name of the package that defined this tool.
func (p Request) PackageOwner() string {
	return p.r.GetToolPackage()
}

func run(ctx context.Context, h AllHandlers) {
	if err := handle(ctx, h); err != nil {
		log.Fatal(err)
	}
}

func Handle(h *Handlers) {
	run(context.Background(), runHandlers{h})
}

func HandleInvoke(f InvokeFunc) {
	h := NewHandlers()
	h.Any().HandleInvoke(f)
	Handle(h)
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
