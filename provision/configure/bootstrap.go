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
	"os"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/logoutput"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/schema"
)

type Request struct {
	Env       *schema.Environment
	Focus     *schema.Stack_Entry
	Stack     *schema.Stack
	Snapshots map[string]fs.FS

	r *protocol.ToolRequest
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

type Handler interface {
	Apply(context.Context, Request, *ApplyOutput) error
	Delete(context.Context, Request, *DeleteOutput) error
}

// XXX remove Tool when all the uses are gone.
type Tool interface {
	Handler
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

func RunTool(t Tool) {
	flag.Parse()

	ctx := logoutput.WithOutput(context.Background(), logoutput.OutputTo{Writer: os.Stderr})

	if err := runTool(ctx, os.Stdin, os.Stdout, t); err != nil {
		log.Fatal(err)
	}
}

func RunWith(h *Handlers) {
	RunTool(runHandlers{h})
}

func runTool(ctx context.Context, r io.Reader, w io.Writer, t Tool) error {
	reqBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	req := &protocol.ToolRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return err
	}

	s := req.Stack.GetServer(schema.PackageName(req.FocusedServer))
	if s == nil {
		return fnerrors.InternalError("%s: focused server not present in the stack", req.FocusedServer)
	}

	p := Request{
		r:         req,
		Env:       req.Env,
		Focus:     s,
		Stack:     req.Stack,
		Snapshots: map[string]fs.FS{},
	}

	for _, snapshot := range req.Snapshot {
		var m memfs.FS
		for _, entry := range snapshot.Entry {
			m.Add(entry.Path, entry.Contents)
		}
		p.Snapshots[snapshot.Name] = &m
	}

	response := &protocol.ToolResponse{}

	switch req.RequestType.(type) {
	case *protocol.ToolRequest_ApplyRequest:
		var out ApplyOutput
		if err := t.Apply(ctx, p, &out); err != nil {
			return err
		}

		response.ApplyResponse = &protocol.ApplyResponse{}
		for _, input := range out.Extensions {
			packed, err := input.ToDefinition()
			if err != nil {
				return err
			}

			if packed.For == "" {
				packed.For = s.GetPackageName().String()
			}

			response.ApplyResponse.Extension = append(response.ApplyResponse.Extension, packed)
		}

		response.ApplyResponse.Definition, err = defs.Make(out.Definitions...)
		if err != nil {
			return err
		}

	case *protocol.ToolRequest_DeleteRequest:
		var out DeleteOutput
		if err := t.Delete(ctx, p, &out); err != nil {
			return err
		}

		response.DeleteResponse = &protocol.DeleteResponse{}
		response.DeleteResponse.Definition, err = defs.Make(out.Ops...)
		if err != nil {
			return err
		}
	}

	serialized, err := proto.Marshal(response)
	if err != nil {
		return err
	}

	w.Write(serialized)
	return nil
}
