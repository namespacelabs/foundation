// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package provisioning

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/std/execution/defs"
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

func (p Request) CheckUnpackInput(msg proto.Message) (bool, error) {
	for _, env := range p.r.Input {
		if env.MessageIs(msg) {
			if msg == nil {
				return false, fnerrors.New("msg is nil")
			}

			return true, env.UnmarshalTo(msg)
		}
	}

	return false, nil
}

func (p Request) UnpackInput(msg proto.Message) error {
	has, err := p.CheckUnpackInput(msg)
	if err == nil && !has {
		return fnerrors.InternalError("no such input: %s", msg.ProtoReflect().Descriptor().FullName())
	}
	return err
}

// PackageOwner returns the name of the package that defined this tool.
func (p Request) PackageOwner() string {
	return p.r.GetToolPackage()
}

func Handle(h *Handlers) {
	if err := HandleInvocation(context.Background(), func(sr grpc.ServiceRegistrar) {
		protocol.RegisterInvocationServiceServer(sr, h.ServiceHandler())
	}); err != nil {
		log.Fatal(err)
	}
}

func HandleInvoke(f InvokeFunc) {
	h := NewHandlers()
	h.Any().HandleInvoke(f)
	Handle(h)
}

func handleRequest(ctx context.Context, req *protocol.ToolRequest, handlers AllHandlers) (*protocol.ToolResponse, error) {
	br := Request{r: req, Snapshots: map[string]fs.FS{}}

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
		if err := handlers.Apply(ctx, p, &out); err != nil {
			return nil, err
		}

		response.ApplyResponse = &protocol.ApplyResponse{}
		for _, ext := range out.ServerExtensions {
			if ext.TargetServer == "" {
				ext.TargetServer = p.Focus.GetPackageName().String()
			}

			response.ApplyResponse.ServerExtension = append(response.ApplyResponse.ServerExtension, ext)
		}

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

		response.ApplyResponse.Invocation, err = defs.Make(out.Invocations...)
		if err != nil {
			return nil, err
		}

		response.ApplyResponse.InvocationSource = out.InvocationSources
		response.ApplyResponse.Computed = out.Computed

		if out.OutputResourceInstance != nil {
			resourceInstance, err := anypb.New(out.OutputResourceInstance)
			if err != nil {
				return nil, err
			}

			resourceJson, err := json.Marshal(out.OutputResourceInstance)
			if err != nil {
				return nil, err
			}

			response.ApplyResponse.OutputResourceInstance = resourceInstance
			response.ApplyResponse.OutputResourceInstanceSerializedJson = resourceJson
		}

		for _, computed := range out.ComputedResourceInput {
			intent, err := json.Marshal(computed.Intent)
			if err != nil {
				return nil, err
			}

			response.ApplyResponse.ComputedResourceInput = append(response.ApplyResponse.ComputedResourceInput, &protocol.ApplyResponse_ResourceInput{
				Name:                 computed.Name,
				Class:                computed.Class,
				SerializedIntentJson: string(intent),
			})
		}

	case *protocol.ToolRequest_DeleteRequest:
		p, err := parseStackRequest(br, x.DeleteRequest.Header)
		if err != nil {
			return nil, err
		}

		var out DeleteOutput
		if err := handlers.Delete(ctx, p, &out); err != nil {
			return nil, err
		}

		response.DeleteResponse = &protocol.DeleteResponse{}
		response.DeleteResponse.Invocation, err = defs.Make(out.Invocations...)
		if err != nil {
			return nil, err
		}

	case *protocol.ToolRequest_InvokeRequest:
		output, err := handlers.Invoke(ctx, br)
		if err != nil {
			return nil, err
		}
		response.InvokeResponse = output
	}

	return response, nil
}
